#!/usr/bin/env python

import argparse
import copy
import datetime
import logging
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time

from multiprocessing.dummy import Pool

CONTAINER_NAME = "claudia"
SRC_ROOT = os.path.realpath(os.path.dirname(__file__))
DEFAULT_BUILDER_IMAGE_NAME = "claudia-builder"
with open(os.path.join(SRC_ROOT, 'VERSION')) as _f:
    VERSION = _f.read().strip()

logger = logging.getLogger('build')
docker_compose_ami_yml_path = os.path.join(SRC_ROOT, "ami/docker-compose-ami.yml.in")
default_components = ['builder', 'server', 'ui', 'container']
all_components = ['builder', 'server', 'ui', 'container', 'ami', 'docs']

def run_cmd(cmd, shell=False, cwd=None, retry=None, retry_interval=None, env=None):
    """Wrapper around subprocess.Popen to capture/print output

    :param shell: execute the command in a shell
    :param retry: number of retries to perform if command fails with non-zero return code
    :param retry_interval: interval between retries
    """
    orig_cmd = cmd
    if not shell:
        cmd = shlex.split(cmd)
    attempts = 1 if not retry else 1 + retry
    retry_interval = retry_interval or 10
    cwd = cwd or SRC_ROOT

    for attempt in range(attempts):
        lines = []
        logger.info('$ %s', orig_cmd)
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, universal_newlines=True,
                                stderr=subprocess.STDOUT, shell=shell, cwd=cwd, env=env)
        for line in iter(proc.stdout.readline, ''):
            line = line[0:-1] # chop the newline
            logger.debug(line)
            lines.append(line)
        proc.stdout.close()
        proc.wait()
        output = '\n'.join(lines)
        if proc.returncode == 0:
            break
        else:
            if attempt+1 < attempts:
                logger.warning(
                    "Attempt %s/%s of command: '%s' failed with returncode %s. Retrying in %ss",
                    attempt+1, attempts, orig_cmd, proc.returncode, retry_interval)
                time.sleep(retry_interval)
    else:
        raise subprocess.CalledProcessError(proc.returncode, cmd, output=output)
    return output

def build_builder():
    """Builds the builder container, with the golang environment that can build this project"""
    dockerfile_path = os.path.join(SRC_ROOT, "Dockerfile-builder")
    out = run_cmd("docker build -t {} -f {} .".format(DEFAULT_BUILDER_IMAGE_NAME, dockerfile_path))
    matches = re.findall(r"Successfully built\s+(\w+)", out)
    image_id = matches[-1]
    return image_id

def build_container_binaries(builder_image):
    """Builds the container binaries, to be copied into the final container"""
    # Map an empty dir to the project's vendor directory to prevent go from using
    # those sources, and force it to use the packages in the builder instead. This will
    # ensure we are not inadvertently pick up any packages from user's directory.
    tmpdir = tempfile.mkdtemp(prefix='emptydir-', dir='/tmp')
    claudia_path = "/root/go/src/github.com/applatix/claudia"
    cmd = "docker run --rm " \
        "-v {src_root}:{claudia_path} " \
        "-v {tmpdir}:{claudia_path}/vendor " \
        "{builder_image} {claudia_path}/build.sh" \
        .format(src_root=SRC_ROOT, claudia_path=claudia_path, tmpdir=tmpdir, builder_image=builder_image)
    try:
        run_cmd(cmd)
    finally:
        shutil.rmtree(tmpdir)

def build_container_static(builder_image):
    """Builds the container static resources, to be copied into the final container"""
    run_cmd("docker run --rm -e VERSION={} -v {}:/src {} /src/ui/build.sh"
            .format(VERSION, SRC_ROOT, builder_image))

def which(program):
    """Tests if a program exists from command line"""
    try:
        run_cmd("which {}".format(program))
        return True
    except subprocess.CalledProcessError:
        pass
    return False

def build_docs(builder_image):
    """Builds the documentation"""
    if which('docker'):
        run_cmd("docker run --rm -v {}:/src --workdir /src {} mkdocs build".format(SRC_ROOT, builder_image))
    elif which('mkdocs'):
        run_cmd("mkdocs build")
    else:
        raise Exception("Either docker or mkdocs needs to be installed to build documentation")
    logger.info("Documentation built at: %s/site", SRC_ROOT)

def build_container_image(image_tag):
    """Builds the the final container image"""
    cmd = "docker build -t {} .".format(image_tag)
    out = run_cmd(cmd)
    matches = re.findall(r"Successfully built\s+(\w+)", out)
    image_id = matches[-1]
    return image_id

def generate_docker_compose_ami_yml(image_tag):
    """Generates the docker-compose.yml to be used in the AMI"""
    with open(docker_compose_ami_yml_path, 'r') as f:
        contents = f.read()
    for macro, val in [("${IMAGE_TAG}", image_tag)]:
        contents = contents.replace(macro, val)
    docker_compose_path = os.path.join(SRC_ROOT, "ami/docker-compose.yml")
    with open(docker_compose_path, 'w') as f:
        f.write(contents)
    logger.info("Generated AMI docker-compose.yml with version %s", VERSION)

def verify_packer_version():
    """Verifies required packer version is installed"""
    packer_version = tuple([int(i) for i in run_cmd("packer --version").split('.')])
    if packer_version < (1, 0, 1):
        logger.error("Packer verison 1.0.1+ required")
        sys.exit(1)

def get_aws_env(args):
    """Returns a dictionary of AWS related environment variables based on command line args"""
    env = {}
    env_values = {
        "AWS_PROFILE": args.aws_profile,
        "AWS_ACCESS_KEY_ID": args.aws_access_key_id,
        "AWS_SECRET_ACCESS_KEY": args.aws_secret_access_key,
        "AWS_SESSION_TOKEN": args.aws_session_token,
    }
    for aws_key, aws_val in env_values.items():
        if aws_val:
            env[aws_key] = aws_val
    return env

def build_ami(image_tag, aws_env):
    """Builds the AMI using packer"""
    generate_docker_compose_ami_yml(image_tag)
    claudia_version = run_cmd("docker run --rm {} claudiad --version".format(image_tag)).strip()
    version_match = re.match(r"^.*\s+(\d+\.\d+\.\d+-\S+)\s+\(Build Date: (.*)\)$", claudia_version)
    full_version = version_match.group(1)
    version = full_version.split('-')[0]
    build_date = datetime.datetime.strptime(version_match.group(2), '%Y-%m-%dT%H:%M:%S')
    build_timestamp = build_date.strftime("%Y%m%d%H%M%S")

    run_cmd("docker save {} | gzip -c > ami/claudia.tar.gz".format(image_tag), shell=True)
    env = copy.deepcopy(os.environ)
    env.update(aws_env)
    run_cmd("packer build -var 'version={}' -var 'full_version={}' -var 'build_timestamp={}' packer.json".format(version, full_version, build_timestamp), env=env)

def argparser():
    """Return an argument parser"""
    parser = argparse.ArgumentParser(description='Claudia build')
    parser.add_argument("-c", "--component", type=str, action='append', help="Build specified component (default: {})".format(default_components))
    parser.add_argument("-nc", "--no-cache", action="store_true", help="Build without cache.")
    parser.add_argument("--release", action="store_true", help="Build in release mode (builds all components, sets container version, and pushes images)")
    parser.add_argument("--all", action="store_true", help="Build all components")
    parser.add_argument("--registry", type=str, help="Tag and push to the given registry (e.g. docker.io)")
    parser.add_argument("--image-version", "-v", type=str, default='latest', help="Tag container with the given version")
    parser.add_argument("--publish", action="store_true", help="Publish docs after building")
    parser.add_argument("--aws-profile", type=str, help="AWS Profile for building AMI")
    parser.add_argument("--aws-access-key-id", type=str, help="AWS Access Key ID for building AMI")
    parser.add_argument("--aws-secret-access-key", type=str, help="AWS Secret Access Key for building AMI")
    parser.add_argument("--aws-session-token", type=str, help="AWS session token for building AMI")
    return parser

def main():
    """Build the builder, the linux binaries, UI, container, and AMI"""
    logging.basicConfig(
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout)
    logging.getLogger("build").setLevel(logging.DEBUG)
    parser = argparser()
    args = parser.parse_args()

    if args.release:
        if not args.registry:
            parser.error("Registry must be supplied during release")
        args.all = True
        args.image_version = VERSION
        logger.info("Building in release mode")

    if args.all:
        args.component = all_components
    elif not args.component:
        args.component = default_components
    if set(args.component) - set(all_components):
        parser.error("Invalid component(s): {}".format(list(set(args.component) - set(all_components))))

    if set(args.component) & set(['builder', 'server', 'ui']):
        builder_image = build_builder()
    else:
        builder_image = DEFAULT_BUILDER_IMAGE_NAME

    if 'ami' in args.component:
        verify_packer_version()

    pool = Pool(2)
    async_results = []
    if 'server' in args.component:
        async_res = pool.apply_async(build_container_binaries, args=(builder_image, ))
        async_results.append(('server', async_res))
    if 'ui' in args.component:
        async_res = pool.apply_async(build_container_static, args=(builder_image, ))
        async_results.append(('ui', async_res))
    pool.close()
    pool.join()
    for _, async_res in async_results:
        # will raise any exception that occurred
        async_res.get()

    if 'docs' in args.component:
        build_docs(builder_image)
    image_tag = "{}:{}".format(CONTAINER_NAME, args.image_version)
    if 'container' in args.component:
        build_container_image(image_tag)
        if args.registry:
            registry_image_tag = "{}/{}".format(args.registry, image_tag)
            run_cmd("docker tag {} {}".format(image_tag, registry_image_tag))
            run_cmd("docker push {}".format(registry_image_tag))
    if 'ami' in args.component:
        aws_env = get_aws_env(args)
        build_ami(image_tag, aws_env)

if __name__ == '__main__':
    main()
