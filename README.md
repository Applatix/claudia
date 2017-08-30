# Claudia

Claudia is a cost and usage analytics solution that provides insights into your AWS cloud spending.

![Claudia Screenshot](docs/report-screenshot.png)

# Build Instructions

### Build Requirements
 * docker
 * python
 * awscli (if publishing docs or building AMI)

### Build Container
```
./build.py
```

### Build Docs
```
./build.py -c docs
```
Optionally publish the docs to S3 bucket. Requires your profile has write permissions to the `ax-public` bucket.

```
./build.py -c docs --publish
```

### Build AMI
The AMI build process uses [packer](https://packer.io) to create new AMI.

```
./build.py --all --aws-profile <profile_name>
```

# Run Instructions

### Running Locally

After building the the container, run:

```
docker-compose up
```

Visit [https://localhost](https://localhost) to access the app.

### Running in debug mode

```
docker-compose -f docker-compose-debug.yml up
```

Debug mode will:

 * Enable InfluxDB's admin interface
 * Expose the following ports:
   * Postgres 5432
   * InfluxDB 8086
   * InfluxDB admin UI 8086
   * ingestd 8081
   * claudiad 80/443
 * Leave the claudia container running in the event that the process dies, for the purpose of bashing into the container to restart the process manually or inspect any files.


### Running in development mode

```
docker-compose -f docker-compose-dev.yml up
```

Development mode starts only the postgres and influxdb containers and exposes their ports. This mode allows you to run ingestd and claudiad manually (e.g. `go run`) and connect to localhost IP addresses.

To run ingestd manually, and connect it to the localhost postgres and influxdb:

```
USERDB_HOST=localhost:5432 POSTGRES_DB=userdb POSTGRES_PASSWORD=my-secret-pw go run ingestd/main.go run --reportDir /tmp/claudia --costdb http://localhost:8086
```

To run claudiad manually, connect it to the localhost postgres, influxdb, and ingestd, while also disabling SSL and running it on a different port (8080):

```
USERDB_HOST=localhost:5432 POSTGRES_DB=userdb POSTGRES_PASSWORD=my-secret-pw go run claudiad/main.go --costdbURL http://localhost:8086 --ingestdURL http://localhost:8081 --assets ./ui/dist/ --insecure --port 8080
```

# Editing Documentation

The documentation site is built using [MkDocs](http://www.mkdocs.org/), a static site generator that creates static documentation from markdown files.

### Requirements
```
pip install mkdocs mkdocs-bootswatch
```

### Editing and previewing changes

Run mkdocs server and visit [http://localhost:8000](http://localhost:8000) to preview your changes.

```
mkdocs serve
```

Edit `mkdocs.yml` to change layout, headers, theme and other global settings. Edit markdown files under the `docs` directory to update and add new content.
