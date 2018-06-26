SET TIME ZONE 'UTC';


DROP DATABASE confluentdb_zendesk;

CREATE DATABASE confluentdb_zendesk
		ENCODING  'UTF8'
		LC_COLLATE 'en_US.UTF8'
		LC_CTYPE 'en_US.UTF8';

CREATE USER zendb with ENCRYPTED password 'password';
GRANT CONNECT ON DATABASE confluentdb_zendesk to zendb;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO zendb;

\c confluentdb_zendesk

CREATE TABLE IF NOT EXISTS organizationfields (
	id			    BIGINT  PRIMARY KEY ,
	type					VARCHAR(255),
	skey				VARCHAR(255) DEFAULT 'UNDEFINED',
	title			    VARCHAR(255),
	createdat	BIGINT DEFAULT  -1,
	updatedat	BIGINT DEFAULT  -1
);

CREATE TABLE IF NOT EXISTS userfields (
	id			    BIGINT  PRIMARY KEY ,
	type					VARCHAR(255) NOT NULL,
	skey				VARCHAR(255) ,
	title			    VARCHAR(255) NOT NULL,
	createdat	BIGINT DEFAULT -1,
	updatedat	BIGINT DEFAULT -1
);

CREATE TABLE IF NOT EXISTS ticketfields (
	id			    BIGINT  PRIMARY KEY ,
	type					VARCHAR(255) NOT NULL,
	skey				VARCHAR(255) DEFAULT 'UNDEFINED',
	title			    VARCHAR(255) NOT NULL,
	createdat	BIGINT DEFAULT -1,
	updatedat	BIGINT DEFAULT -1
);

/* TODO: There are more metrics I want to extract, this will have to suffice for the first iteration */
CREATE TABLE IF NOT EXISTS ticketmetric (
	id         BIGINT  PRIMARY KEY ,
	ticketid BIGINT  NOT NULL,
	createdat 	BIGINT DEFAULT -1,
	updatedat 	BIGINT DEFAULT -1,
	solvedat	 BIGINT  DEFAULT -1,
	assignedat BIGINT DEFAULT -1,
	initiallyassignedat BIGINT DEFAULT -1,
	latestcommentaddedat BIGINT DEFAULT -1,
	assigneeUpdatedat BIGINT DEFAULT -1,
	requesterupdatedat BIGINT DEFAULT -1,
	statusupdatedat BIGINT DEFAULT -1,
	agentwaittime BIGINT DEFAULT -1,
	requesterwaittime BIGINT DEFAULT -1,
	reopens		 BIGINT  DEFAULT 0,
	replies		 BIGINT  DEFAULT 0,
	ttfr			 BIGINT	DEFAULT 0,
	ttr				 BIGINT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS groups (
	id          BIGINT PRIMARY KEY,
	name        VARCHAR(50) DEFAULT 'UNDEFINED',
	createdat  BIGINT DEFAULT -1,
	updatedat  BIGINT	DEFAULT -1
);

/* group id is not mandatory for organization */
INSERT INTO groups(id) VALUES (0);

CREATE TABLE IF NOT EXISTS organization (
	id          BIGINT PRIMARY KEY,
	externalID	VARCHAR(255) DEFAULT 'UNDEFINED',
	name        VARCHAR(255) DEFAULT 'UNDEFINED',
	createdat   BIGINT DEFAULT -1,
	updatedat  BIGINT DEFAULT -1,
	groupid	  BIGINT  NOT NULL,
	FOREIGN KEY (groupid)
		REFERENCES groups(id)
);

/* some users do not have an organization id despite having an org mapping */
INSERT INTO organization (id, groupid) VALUES(0, 0);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS organizationdata (
	objectid BIGINT PRIMARY KEY ,
	id  BIGINT  NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(1024),
	transformed VARCHAR(1024)
);

CREATE TABLE IF NOT EXISTS users (
	id                BIGINT PRIMARY KEY,
	externalid				VARCHAR(255) DEFAULT 'UNDEFINED',
	email	            VARCHAR(255) DEFAULT 'UNDEFINED',
	name              VARCHAR(255) DEFAULT 'UNDEFINED',
	createdat	  		BIGINT DEFAULT -1,
	updatedat			  BIGINT DEFAULT -1,
	lastloginat			BIGINT DEFAULT -1,
	organizationid		BIGINT  DEFAULT 0,
	groupid						BIGINT  DEFAULT 0,
	role				      VARCHAR(255) DEFAULT 'UNDEFINED',
	suspended					BOOL DEFAULT TRUE,
	timezone			    VARCHAR(255) DEFAULT 'UNDEFINED',
	FOREIGN KEY (organizationid)
		REFERENCES organization(id),
	FOREIGN KEY (groupid)
		REFERENCES groups(id)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS userdata (
	objectid BIGINT PRIMARY KEY,
	id  BIGINT  NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(255),
	transformed VARCHAR(255)
);

INSERT INTO users(id) VALUES(0);

CREATE TABLE IF NOT EXISTS ticket (
	id				  		BIGINT PRIMARY KEY,
	externalid			VARCHAR(255) DEFAULT 'UNDEFINED',
	subject	        VARCHAR(255) NOT NULL,
	status          VARCHAR(10) NOT NULL,
	requesterid		BIGINT  DEFAULT 0,
	submitterid		BIGINT  DEFAULT 0,
	assigneeid     BIGINT DEFAULT 0,
	recipient			VARCHAR(255) DEFAULT 'UNDEFINED',
	organizationid BIGINT DEFAULT 0,
	groupid        BIGINT DEFAULT 0,
	createdat      BIGINT DEFAULT -1,
	updatedat      BIGINT DEFAULT -1,
	FOREIGN KEY (requesterid)
		REFERENCES users(id),
	FOREIGN KEY (submitterid)
		REFERENCES users(id),
	FOREIGN KEY (assigneeid)
		REFERENCES users(id),
	FOREIGN KEY (organizationid)
		REFERENCES organization(id),
	FOREIGN KEY (groupid)
		REFERENCES groups(id)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS ticketdata (
	objectid BIGINT PRIMARY KEY,
	id  BIGINT NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(255),
	transformed VARCHAR(255)
);

/* This only considers change events and their current value */
CREATE TABLE IF NOT EXISTS audit (
	id	BIGINT  PRIMARY KEY,
	ticketid BIGINT NOT NULL,
	createdat BIGINT DEFAULT -1,
	authorid BIGINT NOT NULL,
	FOREIGN KEY (ticketid)
		REFERENCES ticket(id),
	FOREIGN KEY (authorid)
	REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS changeevent (
	id        BIGINT  PRIMARY KEY ,
	auditid   BIGINT NOT NULL,
	type 			VARCHAR(255),
	fieldname VARCHAR(255),
	value			BIGINT DEFAULT -1,
	pvalue		BIGINT DEFAULT -1,
	FOREIGN KEY (auditid)
		REFERENCES audit(id)
);

CREATE TABLE IF NOT EXISTS satisfactionrating (
	id          BIGINT  UNIQUE,
	assigneeid  BIGINT NOT NULL,
	groupid     BIGINT  NOT NULL,
	requesterid BIGINT NOT NULL,
	ticketid    BIGINT  NOT NULL,
	score       VARCHAR(255) ,
	createdat   BIGINT DEFAULT -1,
	updatedat   BIGINT DEFAULT -1,
	reason     VARCHAR(1024),
	PRIMARY KEY( id, ticketid),
	FOREIGN KEY (ticketid)
		REFERENCES ticket(id),
	FOREIGN KEY (requesterid)
		REFERENCES users(id),
	FOREIGN KEY (assigneeid)
		REFERENCES users(id),
	FOREIGN KEY (groupid)
	REFERENCES groups(id)
);

create index auditKey on audit (ticketId);
create index metricKey on ticketmetric (ticketId);
create index organizationName on organization (name);

/* For public consumption */
CREATE OR REPLACE VIEW TimeSpent AS SELECT ticket.id, max(changeevent.value) AS value
	FROM ticket JOIN audit ON ticket.id = audit.ticketid
		JOIN changeevent ON changeevent.auditid = audit.id
	GROUP BY ticket.id ORDER BY ticket.id;

CREATE OR REPLACE VIEW TicketTime AS SELECT TimeSpent.id AS ticketid, TimeSpent.value AS tickettime
																		 FROM TimeSpent ORDER BY TimeSpent.id;

CREATE OR REPLACE VIEW TicketPriority AS SELECT ticketdata.objectid AS ticketid, ticketdata.value AS priority
															FROM ticketdata WHERE title = 'Case Priority' ORDER BY objectID;

CREATE OR REPLACE VIEW TicketComponent AS SELECT ticketdata.objectid AS ticketid, ticketdata.value AS component
															FROM ticketdata WHERE title = 'Component' ORDER BY objectID;

CREATE OR REPLACE VIEW TicketCause AS SELECT ticketdata.objectid AS ticketid, ticketdata.value AS cause
													FROM ticketdata WHERE title = 'Root Cause' ORDER BY objectID;

CREATE OR REPLACE VIEW TicketVersion AS SELECT ticketdata.objectid AS ticketid, ticketdata.value AS version
													 FROM ticketdata WHERE title = 'Confluent/Kafka Version' ORDER BY objectID;

CREATE OR REPLACE VIEW BundleUsage AS SELECT ticketdata.objectid AS ticketid, ticketdata.value AS bundleused
													 FROM ticketdata WHERE title = 'Support Bundle Used' ORDER BY objectID;

CREATE OR REPLACE VIEW TicketView AS
	SELECT ticket.id, organization.name as organization, to_timestamp(ticket.createdat) as created, to_timestamp(ticket.updatedat) as updated,
		ticket.status, ticket.subject,TicketPriority.priority, TicketComponent.component, TicketTime.ticketTime, TicketCause.cause,
		TicketVersion.version, BundleUsage.bundleused, ticketmetric.ttfr, ticketmetric.ttr, to_timestamp(ticketmetric.solvedat) as solved,
		ticketmetric.agentwaittime,ticketmetric.requesterwaittime
FROM ticket
	JOIN organization on ticket.organizationid = organization.id
	JOIN users on ticket.assigneeid = users.id
	JOIN TicketPriority ON ticket.id = TicketPriority.ticketid
	JOIN TicketComponent ON ticket.id = TicketComponent.ticketid
	JOIN TicketCause ON ticket.id = TicketCause.ticketid
	JOIN TicketVersion ON ticket.id = TicketVersion.ticketid
	JOIN BundleUsage ON ticket.id = BundleUsage.ticketid LEFT OUTER
	JOIN ticketmetric ON ticket.id = ticketmetric.ticketid LEFT OUTER
	JOIN TicketTime ON ticket.id = TicketTime.ticketid
	ORDER BY ticket.id;

CREATE OR REPLACE VIEW OrganizationTam AS SELECT organizationdata.objectid AS organizationid, organizationdata.value as tam
	FROM organizationdata WHERE title = 'technical_account_manager' ORDER BY objectid;

CREATE OR REPLACE VIEW OrganizationRenewal AS SELECT organizationdata.objectid AS organizationid, organizationdata.value as renewaldate
															 FROM organizationdata WHERE title = 'renewal_date' ORDER BY objectid;

CREATE OR REPLACE VIEW OrganizationTZ AS SELECT organizationdata.objectid AS organizationid, organizationdata.value as timezone
																	 FROM organizationdata WHERE title = 'primary_timezone' ORDER BY objectid;

CREATE OR REPLACE VIEW OrganizationEntitlement AS SELECT organizationdata.objectid AS organizationid, organizationdata.value as entitlement
															FROM organizationdata WHERE title = 'subscription_type' ORDER BY objectid;

CREATE OR REPLACE VIEW OrganizationSE AS SELECT organizationdata.objectid AS organizationid, organizationdata.value as se
															FROM organizationdata WHERE title = 'systems_engineer' ORDER BY objectid;

CREATE OR REPLACE VIEW OrganizationView AS
	SELECT organization.*, OrganizationEntitlement.entitlement, OrganizationRenewal.renewaldate, OrganizationSE.se,
		OrganizationTam.tam, OrganizationTZ.timezone
	FROM organization
		JOIN OrganizationTam ON organization.id = OrganizationTam.organizationid
		JOIN OrganizationRenewal ON organization.id = OrganizationRenewal.organizationid
		JOIN OrganizationTZ ON organization.id = OrganizationTZ.organizationid
		JOIN OrganizationSE ON organization.id = OrganizationSE.organizationid
		JOIN OrganizationEntitlement ON organization.id = OrganizationEntitlement.organizationid
	ORDER BY organization.id;

