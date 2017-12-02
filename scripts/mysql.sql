SET GLOBAL time_zone = '+00:00';

CREATE DATABASE IF NOT EXISTS zendb
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

GRANT ALL PRIVILEGES ON zendb.* TO 'zendb'@'%' IDENTIFIED BY 'password';

USE zendb;

CREATE TABLE IF NOT EXISTS sequence_table (
	sequence_name       VARCHAR(20) UNIQUE KEY NOT NULL,
	last_val            BIGINT UNSIGNED NOT NULL DEFAULT 0,
	PRIMARY KEY (`sequence_name`)
);



CREATE TABLE IF NOT EXISTS organization_fields (
	id			    BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	sid			    VARCHAR(255) UNIQUE NOT NULL,
	title		    VARCHAR(30) NOT NULL,
	created_at	INT UNSIGNED NOT NULL, 
	updated_at	INT UNSIGNED NOT NULL,
	PRIMARY KEY(`id`)
);

CREATE TABLE IF NOT EXISTS user_fields (
	id			    BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	sid			    VARCHAR(255) UNIQUE NOT NULL,
	title		    VARCHAR(30) NOT NULL,
	created_at	INT UNSIGNED NOT NULL, 
	updated_at	INT	UNSIGNED NOT NULL, 
	PRIMARY KEY(`id`)	
);

CREATE TABLE IF NOT EXISTS ticket_fields (
	id					BIGINT UNSIGNED UNIQUE KEY NOT NULL, 
	title				VARCHAR(30) NOT NULL,
	PRIMARY KEY (`id`)
);

/* TODO: There are more metrics I want to extract, this will have to suffice for the first iteration */
CREATE TABLE IF NOT EXISTS ticket_metrics (
  id         BIGINT UNSIGNED UNIQUE KEY NOT NULL,
  created_at BIGINT UNSIGNED,
  updated_at BIGINT UNSIGNED,
  ticket_id	 BIGINT UNSIGNED NOT NULL,
  replies		 BIGINT UNSIGNED,
	ttfr			 BIGINT	UNSIGNED,
  solved_at	 BIGINT UNSIGNED DEFAULT 0,
  PRIMARY KEY(`id`)
);

CREATE TABLE IF NOT EXISTS groups (
	id          BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	name        VARCHAR(50) NOT NULL,
	created_at  INT UNSIGNED NOT NULL,
	updated_at  INT	UNSIGNED NOT NULL,
	PRIMARY KEY(`id`)
);

/* group id is not mandatory for organizations. I still want them mapped for future cases */
INSERT INTO groups VALUES (0, "UNDEFINED", 0, 0);

CREATE TABLE IF NOT EXISTS organizations (
	id          BIGINT	UNSIGNED UNIQUE KEY	NOT NULL,
	name        VARCHAR(255) NOT NULL,
	created_at   INT UNSIGNED NOT NULL,
	updated_at  INT	UNSIGNED NOT NULL,
	group_id	  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`id`),
	FOREIGN KEY (`group_id`)
		REFERENCES groups(`id`)
);

/* some users do not have an organization id despite having an org mapping */
INSERT INTO organizations VALUES( 0, "UNDEFINED", 0, 0, 0);

CREATE TABLE IF NOT EXISTS users (
	id                BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	email	            VARCHAR(255) NOT NULL,
	name              VARCHAR(255) NOT NULL,
	created_at	  		INT UNSIGNED NOT NULL,
	organization_id		BIGINT UNSIGNED DEFAULT 0,
	default_group_id	BIGINT UNSIGNED NOT NULL, 
	role				      VARCHAR(10)	NOT NULL,
	time_zone			    VARCHAR(30)	NOT NULL,
	updated_at			  INT UNSIGNED NOT NULL,
	PRIMARY KEY (`id`),
	FOREIGN KEY (`organization_id`) 
		REFERENCES organizations(`id`),
	FOREIGN KEY (`default_group_id`)
		REFERENCES groups(`id`)
);

CREATE TABLE IF NOT EXISTS tickets (
	id				  		BIGINT UNSIGNED UNIQUE KEY NOT NULL,
	subject	        VARCHAR(255) NOT NULL,
	status          VARCHAR(10) NOT NULL,
	requester_id		BIGINT UNSIGNED NOT NULL,
	submitter_id		BIGINT UNSIGNED NOT NULL,
	assignee_id     BIGINT UNSIGNED NOT NULL,
	organization_id BIGINT UNSIGNED DEFAULT 0,
	group_id        BIGINT UNSIGNED NOT NULL,
	created_at      INT UNSIGNED NOT NULL,
	updated_at      INT UNSIGNED NOT NULL,
	version		    	VARCHAR(55) DEFAULT '-',
  component       VARCHAR(55) DEFAULT '-',
  priority        VARCHAR(10) DEFAULT 'undefined',
	ttfr						BIGINT UNSIGNED,
	solved_at       BIGINT UNSIGNED DEFAULT 0,
 	PRIMARY KEY (`id`),
	FOREIGN KEY (`requester_id`)
		REFERENCES users(`id`),
	FOREIGN KEY (`submitter_id`)
		REFERENCES users(`id`),
	FOREIGN KEY (`assignee_id`)
		REFERENCES users(`id`),
	FOREIGN KEY (`organization_id`)
		REFERENCES organizations(`id`),
	FOREIGN KEY (`group_id`)
		REFERENCES groups(`id`)
);

/* holding place for flattening custom fields */
CREATE TABLE IF NOT EXISTS ticket_metadata (
	ticket_id BIGINT UNSIGNED NOT NULL,
	field_id  BIGINT  UNSIGNED NOT NULL,
	raw_value     VARCHAR(255),
	transformed_value VARCHAR(255),
	PRIMARY KEY (`ticket_id`, `field_id`),
	FOREIGN KEY (`field_id`)
		REFERENCES ticket_fields(`id`)
);

CREATE TABLE IF NOT EXISTS ticket_audits (
	ticket_id BIGINT UNSIGNED NOT NULL,
	author_id BIGINT UNSIGNED NOT NULL,
	value     BIGINT UNSIGNED,
	PRIMARY KEY (`ticket_id`),
	FOREIGN KEY (`ticket_id`)
		REFERENCES tickets(`id`),
	FOREIGN KEY (`author_id`)
	REFERENCES users(`id`)
);

/* convenience table */
CREATE VIEW ticket_view AS SELECT tickets.id, tickets.priority, organizations.name AS organization, users.name AS requester,
                             tickets.status, tickets.component, tickets.version, FROM_UNIXTIME(tickets.created_at) AS created_at,
                             FROM_UNIXTIME(tickets.solved_at) AS solved_at
                           FROM tickets
                              JOIN organizations ON tickets.organization_id = organizations.id
                              JOIN users ON tickets.requester_id = users.id;


/* Ensure last id always increments, this couples us to the DB but simplifies code */
DELIMITER //
CREATE TRIGGER increment_only BEFORE UPDATE ON zendb.sequence_table FOR EACH ROW
  BEGIN
    IF OLD.last_val > NEW.last_val THEN
      SET NEW.last_val = OLD.last_val;
    ELSE
      SET NEW.last_val = NEW.last_val + 1;
    END IF;
  END
//

DELIMITER //
CREATE TRIGGER increment_audit BEFORE UPDATE ON zendb.ticket_audits FOR EACH ROW
	BEGIN
		IF OLD.value > NEW.value THEN
			SET NEW.value = OLD.value;
		ELSE
			SET NEW.value = NEW.value + 1;
		END IF;
	END
//