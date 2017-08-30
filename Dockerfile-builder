# This container creates the claudia builder image, which contains the environment necessary to build this project (e.g. golang, node)

FROM debian:9.1

# Install python, git, packer, mkdocs
# NOTE: python2 is installed over python3 because python2 eventually gets installed
# by the nodejs setup script anyways, and mkdocs prefers python2 over python3
# because of click incompatibilites.
RUN apt-get update && apt-get install -y \
    wget \
    python-minimal \
    git \
    curl \
    gcc \
    gnupg \
    unzip && \
    curl https://bootstrap.pypa.io/get-pip.py | python && \
    pip install --no-cache-dir mkdocs==0.16.1 && \
    wget https://releases.hashicorp.com/packer/0.12.2/packer_0.12.2_linux_amd64.zip && \
    unzip -d /usr/bin packer*.zip && \
    rm -f packer*.zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install docker (necessary for using this image as the build container during Argo CI)
ENV DOCKER_BUCKET get.docker.com
ENV DOCKER_VERSION 1.11.2
ENV DOCKER_SHA256 8c2e0c35e3cda11706f54b2d46c2521a6e9026a7b13c7d4b8ae1f3a706fc55e1

RUN set -x \
	&& curl -fSL "https://${DOCKER_BUCKET}/builds/Linux/x86_64/docker-${DOCKER_VERSION}.tgz" -o docker.tgz \
	&& echo "${DOCKER_SHA256} *docker.tgz" | sha256sum -c - \
	&& tar -xzvf docker.tgz \
	&& mv docker/* /usr/local/bin/ \
	&& rmdir docker \
	&& rm docker.tgz \
	&& docker -v

# Install go
ENV GO_VERSION 1.8.3
ENV GO_ARCH amd64
RUN wget https://storage.googleapis.com/golang/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz && \
    tar -C /usr/local/ -xf /go${GO_VERSION}.linux-${GO_ARCH}.tar.gz && \
    rm /go${GO_VERSION}.linux-${GO_ARCH}.tar.gz 

# Install nodejs
RUN curl -sL https://deb.nodesource.com/setup_7.x | bash - && apt-get install -y nodejs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV GOPATH /root/go
ENV PATH ${GOPATH}/bin:/usr/local/go/bin:${PATH}
   
# Install glide
RUN mkdir -p ${GOPATH}/bin && \
    mkdir -p ${GOPATH}/pkg && \
    mkdir -p ${GOPATH}/src && \
    wget https://glide.sh/get && \
    chmod ugo+x ./get && \
    ./get && \
    rm -f get && \
    rm -rf /tmp/glide*

# Install Go dependencies and some tooling
COPY glide.yaml ${GOPATH}
COPY glide.lock ${GOPATH}
RUN cd ${GOPATH} && \
    glide install && \
    mv vendor/* src/ && \
    rmdir vendor && \
    go get -u -v github.com/derekparker/delve/cmd/dlv && \
    go get -u -v gopkg.in/alecthomas/gometalinter.v1 && \
    cd ${GOPATH}/bin && \
    ln -s gometalinter.v1 gometalinter && \
    gometalinter --install

# Install Node dependencies
COPY ui/package.json /root/node/package.json
COPY ui/npm-shrinkwrap.json /root/node/npm-shrinkwrap.json
RUN cd /root/node && npm install

# MkDocs Themes
RUN pip install mkdocs-bootswatch
