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

/* A lot of defaults to accomadate deleted stuff kind of breaks referential integrity */
CREATE TABLE IF NOT EXISTS ticket (
	id				  		BIGINT UNSIGNED NOT NULL,
	externalid			VARCHAR(255) DEFAULT "UNDEFINED",
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
	value     BIGINT UNSIGNED,
	PRIMARY KEY (`id`, `ticketid`),
	FOREIGN KEY (`ticketid`)
		REFERENCES ticket(`id`),
	FOREIGN KEY (`authorid`)
	REFERENCES user(`id`)
);

/* convenience table */
# CREATE VIEW ticketview AS SELECT ticket.id, ticket.priority, organization.name AS organization, user.name AS requester,
#                              ticket.status, ticket.component, ticket.version, FROM_UNIXTIME(ticket.createdat) AS createdat,
# 														 FROM_UNIXTIME(ticket.updatedat) AS updatedat, ticketmetric.ttfr,
# 														 FROM_UNIXTIME(ticketmetric.solvedat) AS solvedat
#                            FROM ticket
#                               JOIN organization ON ticket.organizationid = organization.id
#                               JOIN user ON ticket.requesterid = user.id
# 															JOIN ticketmetric on ticket.id = ticketmetric.ticketid;
#

/* Ensure last id always increments */
DELIMITER //
CREATE TRIGGER incrementonly BEFORE UPDATE ON zendb.sequencetable FOR EACH ROW
  BEGIN
    IF OLD.lastval > NEW.lastval THEN
      SET NEW.lastval = OLD.lastval;
    ELSE
      SET NEW.lastval = NEW.lastval + 1;
    END IF;
  END
//

/* Ensure time tracking value always increments */
DELIMITER //
CREATE TRIGGER incrementaudit BEFORE UPDATE ON zendb.audit FOR EACH ROW
	BEGIN
		IF OLD.value > NEW.value THEN
			SET NEW.value = OLD.value;
		ELSE
			SET NEW.value = NEW.value + 1;
		END IF;
	END
//