
SET GLOBAL TIME_ZONE  = '+00:00';

CREATE DATABASE IF NOT EXISTS zendb
		CHARACTER SET utf8mb4
		COLLATE utf8mb4_unicode_ci;

GRANT ALL PRIVILEGES ON zendb.* TO 'zendb'@'%' IDENTIFIED BY 'password';

USE zendb;

CREATE TABLE IF NOT EXISTS sequencetable (
	sequencename       VARCHAR(20) UNIQUE KEY NOT NULL,
	lastval            BIGINT UNSIGNED NOT NULL DEFAULT 0,
	PRIMARY KEY (`sequencename`)
);

CREATE TABLE IF NOT EXISTS organizationfields (
	id			    BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	type					VARCHAR(255) NOT NULL,
	skey				VARCHAR(255) DEFAULT "UNDEFINED",
	title			    VARCHAR(255) NOT NULL,
	createdat	BIGINT DEFAULT -1,
	updatedat	BIGINT DEFAULT -1,
	PRIMARY KEY(`id`)
);

CREATE TABLE IF NOT EXISTS userfields (
	id			    BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	type					VARCHAR(255) NOT NULL,
	skey				VARCHAR(255) DEFAULT "UNDEFINED",
	title			    VARCHAR(255) NOT NULL,
	createdat	BIGINT DEFAULT -1,
	updatedat	BIGINT DEFAULT -1,
	PRIMARY KEY(`id`)
);

CREATE TABLE IF NOT EXISTS ticketfields (
	id			    BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	type					VARCHAR(255) NOT NULL,
	skey				VARCHAR(255) DEFAULT "UNDEFINED",
	title			    VARCHAR(255) NOT NULL,
	createdat	BIGINT DEFAULT -1,
	updatedat	BIGINT DEFAULT -1,
	PRIMARY KEY(`id`)
);

/* TODO: There are more metrics I want to extract, this will have to suffice for the first iteration */
CREATE TABLE IF NOT EXISTS ticketmetric (
	id         BIGINT UNSIGNED NOT NULL,
	ticketid BIGINT UNSIGNED NOT NULL,
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
	reopens		 BIGINT UNSIGNED DEFAULT 0,
	replies		 BIGINT UNSIGNED DEFAULT 0,
	ttfr			 BIGINT UNSIGNED	DEFAULT 0,
	ttr				 BIGINT UNSIGNED DEFAULT 0,
	PRIMARY KEY(`id`)
# 	FOREIGN KEY (`ticketid`)
# 		REFERENCES ticket(`id`)
	# TODO: figure out why this isn't working
);

CREATE TABLE IF NOT EXISTS groups (
	id          BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	name        VARCHAR(50) DEFAULT "UNDEFINED",
	createdat  BIGINT DEFAULT -1,
	updatedat  BIGINT	DEFAULT -1,
	PRIMARY KEY(`id`)
);

/* group id is not mandatory for organization */
INSERT INTO groups(id) VALUES (0);

CREATE TABLE IF NOT EXISTS organization (
	id          BIGINT	UNSIGNED UNIQUE KEY	NOT NULL,
	externalID	VARCHAR(255) DEFAULT "UNDEFINED",
	name        VARCHAR(255) DEFAULT "UNDEFINED",
	createdat   BIGINT DEFAULT -1,
	updatedat  BIGINT DEFAULT -1,
	groupid	  BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (`id`),
	FOREIGN KEY (`groupid`)
		REFERENCES groups(`id`)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS organizationdata (
	objectid BIGINT UNSIGNED NOT NULL,
	id  BIGINT UNSIGNED NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(1024),
	transformed VARCHAR(1024),
	PRIMARY KEY (`objectid`, `id`)
);

/* some user do not have an organization id despite having an org mapping */
INSERT INTO organization (id, groupid) VALUES(0, 0);

CREATE TABLE IF NOT EXISTS user (
	id                BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	externalid				VARCHAR(255) DEFAULT "UNDEFINED",
	email	            VARCHAR(255) DEFAULT "UNDEFINED",
	name              VARCHAR(255) DEFAULT "UNDEFINED",
	createdat	  		BIGINT DEFAULT -1,
	updatedat			  BIGINT DEFAULT -1,
	lastloginat			BIGINT DEFAULT -1,
	organizationid		BIGINT UNSIGNED DEFAULT 0,
	groupid						BIGINT UNSIGNED DEFAULT 0,
	role				      VARCHAR(255) DEFAULT "UNDEFINED",
	suspended					BOOL DEFAULT TRUE,
	timezone			    VARCHAR(255) DEFAULT "UNDEFINED",
	PRIMARY KEY (`id`),
	FOREIGN KEY (`organizationid`)
		REFERENCES organization(`id`),
	FOREIGN KEY (`groupid`)
		REFERENCES groups(`id`)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS userdata (
	objectid BIGINT UNSIGNED NOT NULL,
	id  BIGINT UNSIGNED NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(255),
	transformed VARCHAR(255),
	PRIMARY KEY (`objectid`, `id`)
);

INSERT INTO user (id) VALUES(0);

CREATE TABLE IF NOT EXISTS ticket (
	id				  		BIGINT UNSIGNED NOT NULL,
	externalid			VARCHAR(255) DEFAULT "UNDEFINED",
  url             VARCHAR(255) DEFAULT "UNDEFINED",
	subject	        VARCHAR(255) NOT NULL,
	status          VARCHAR(10) NOT NULL,
	requesterid		BIGINT UNSIGNED DEFAULT 0,
	submitterid		BIGINT UNSIGNED DEFAULT 0,
	assigneeid     BIGINT UNSIGNED DEFAULT 0,
	recipient			VARCHAR(255) DEFAULT "UNDEFINED",
	organizationid BIGINT UNSIGNED DEFAULT 0,
	groupid        BIGINT UNSIGNED DEFAULT 0,
	createdat      BIGINT DEFAULT -1,
	updatedat      BIGINT DEFAULT -1,
	PRIMARY KEY (`id`),
	FOREIGN KEY (`requesterid`)
		REFERENCES user(`id`),
	FOREIGN KEY (`submitterid`)
		REFERENCES user(`id`),
	FOREIGN KEY (`assigneeid`)
		REFERENCES user(`id`),
	FOREIGN KEY (`organizationid`)
		REFERENCES organization(`id`),
	FOREIGN KEY (`groupid`)
		REFERENCES groups(`id`)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS ticketdata (
	objectid BIGINT UNSIGNED NOT NULL,
	id  BIGINT UNSIGNED NOT NULL,
	title  VARCHAR(255),
	value     VARCHAR(255),
	transformed VARCHAR(255),
	PRIMARY KEY (`objectid`, `id`)
);

/* This only considers change events and their current value */
CREATE TABLE IF NOT EXISTS audit (
	id	BIGINT UNSIGNED UNIQUE,
	ticketid BIGINT UNSIGNED NOT NULL,
	createdat BIGINT DEFAULT -1,
	authorid BIGINT UNSIGNED NOT NULL,
	PRIMARY KEY (`id`, `ticketid`),
	FOREIGN KEY (`ticketid`)
		REFERENCES ticket(`id`),
	FOREIGN KEY (`authorid`)
	REFERENCES user(`id`)
);

CREATE TABLE IF NOT EXISTS changeevent (
	id        BIGINT UNSIGNED UNIQUE,
	auditid   BIGINT UNSIGNED NOT NULL,
	type 			VARCHAR(255),
	fieldname VARCHAR(255),
	value		VARCHAR(255),
	pvalue		VARCHAR(255),
	PRIMARY KEY (`id`, `auditid`),
	FOREIGN KEY (`auditid`)
		REFERENCES audit(`id`)
);

CREATE TABLE IF NOT EXISTS satisfactionrating (
	id          BIGINT UNSIGNED UNIQUE,
	assigneeid  BIGINT UNSIGNED NOT NULL,
	groupid     BIGINT UNSIGNED NOT NULL,
	requesterid BIGINT UNSIGNED NOT NULL,
	ticketid    BIGINT UNSIGNED NOT NULL,
	score       VARCHAR(255) ,
	createdat   BIGINT DEFAULT -1,
	updatedat   BIGINT DEFAULT -1,
	reason     VARCHAR(1024),
	PRIMARY KEY( `id`, `ticketid`),
	FOREIGN KEY (`ticketid`)
		REFERENCES ticket(`id`),
	FOREIGN KEY (`requesterid`)
		REFERENCES user(`id`),
	FOREIGN KEY (`assigneeid`)
		REFERENCES user(`id`),
	FOREIGN KEY (`groupid`)
	REFERENCES groups(`id`)
);

create index auditKey on audit (ticketId);
create index metricKey on ticketmetric (ticketId);

/* For public consumption */
CREATE OR REPLACE VIEW TimeSpent AS SELECT ticket.id, max(cast(changeevent.value AS UNSIGNED INTEGER )) AS value
	FROM ticket JOIN audit ON ticket.id = audit.ticketid
		JOIN changeevent ON changeevent.auditid = audit.id
                JOIN ticketfields on ticketfields.id = changeevent.fieldname
        where ticketfields.title = "Total time spent (sec)"
	GROUP BY ticket.id ORDER BY ticket.id;

CREATE OR REPLACE VIEW TicketTime AS SELECT TimeSpent.id AS ticketid, TimeSpent.value AS tickettime
																		 FROM TimeSpent ORDER BY TimeSpent.id;

CREATE OR REPLACE VIEW TicketInitialPriority AS select audit.ticketid as ticketid, pvalue as initialpriority from changeevent join audit on audit.id=changeevent.auditid join 
(select min(changeevent.id) as firstChanged, audit.ticketid as ticketid from changeevent join ticketfields on changeevent.fieldname = ticketfields.id join audit on audit.id=changeevent.auditid where ticketfields.title="Case Priority" and pvalue > "" group by audit.ticketid) as f on f.firstChanged=changeevent.id;

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
	SELECT ticket.id, organization.name as organization, FROM_UNIXTIME(ticket.createdat) as created, FROM_UNIXTIME(ticket.updatedat) as updated,
		ticket.status, ticket.subject,TicketPriority.priority, ifnull(TicketInitialPriority.initialpriority,TicketPriority.priority) initialpriority,
                TicketComponent.component, TicketTime.ticketTime, TicketCause.cause,
		TicketVersion.version, BundleUsage.bundleused, ticketmetric.ttfr, ticketmetric.ttr, FROM_UNIXTIME(ticketmetric.solvedat) as solved,
		ticketmetric.agentwaittime,ticketmetric.requesterwaittime
FROM ticket
	JOIN organization on ticket.organizationid = organization.id
	JOIN user on ticket.assigneeid = user.id
	JOIN TicketPriority ON ticket.id = TicketPriority.ticketid
	LEFT JOIN TicketInitialPriority ON ticket.id = TicketInitialPriority.ticketid
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
