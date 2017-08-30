// Copyright 2017 Applatix, Inc.
package userdb

// SchemaVersion is the user database schema version of this version of the app
const SchemaVersion = 1

var schemaV1 = []string{`
-- single row table to store configuration & system information
CREATE TABLE configuration (
	id                 INT NOT NULL PRIMARY KEY DEFAULT 1,
	schema_version     INT NOT NULL,
	session_auth_key   BYTEA NOT NULL,
	session_crypt_key  BYTEA NOT NULL,
	private_key        TEXT NOT NULL,
	public_certificate TEXT NOT NULL,
	eula_accepted      BOOLEAN NOT NULL DEFAULT false,
	CONSTRAINT single_row CHECK (id = 1)
);
`, `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
`, `
CREATE EXTENSION IF NOT EXISTS "citext";
`, `
CREATE TABLE appuser (
	id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	ctime          TIMESTAMP NOT NULL DEFAULT current_timestamp,
	mtime          TIMESTAMP NOT NULL DEFAULT current_timestamp,
	username       CITEXT NOT NULL UNIQUE,
	password_hash  TEXT NOT NULL
);
`, `
CREATE TABLE report (
	id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	ctime              TIMESTAMP NOT NULL DEFAULT current_timestamp,
	mtime              TIMESTAMP NOT NULL DEFAULT current_timestamp,
	report_name        CITEXT NOT NULL,
	retention_days     INT NOT NULL,
	status             TEXT NOT NULL,
	status_detail      TEXT NOT NULL,
	-- currently only one report per user is supported. remove unique constraint when this restriction is lifted
	owner_user_id      UUID NOT NULL UNIQUE REFERENCES appuser(id) ON DELETE CASCADE
);
`, `
CREATE TABLE bucket (
	id                     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	ctime                  TIMESTAMP NOT NULL DEFAULT current_timestamp,
	report_id              UUID NOT NULL REFERENCES report(id) ON DELETE CASCADE,
	bucketname             TEXT NOT NULL,
	region                 TEXT NOT NULL,
	report_path            TEXT NOT NULL,
	aws_access_key_id      TEXT NOT NULL,
	aws_secret_access_key  TEXT NOT NULL,
	CONSTRAINT unique_s3path UNIQUE (bucketname, report_path)
);
`, `
-- Table of aws account ids associated with a report. Used primarily translating aws account ids to user defined display names
CREATE TABLE aws_account (
	report_id              UUID NOT NULL REFERENCES report(id) ON DELETE CASCADE,
	aws_account_id         TEXT NOT NULL,
	name                   TEXT NOT NULL,
	CONSTRAINT unique_account UNIQUE (report_id, aws_account_id)
);
`, `
-- Table of product codes mapped to AWS product information about the product. This is global information, not tied to a specific user
CREATE TABLE aws_product (
	product_code           TEXT NOT NULL UNIQUE,
	name                   TEXT NOT NULL,
	description            TEXT NOT NULL
);
`,
}
