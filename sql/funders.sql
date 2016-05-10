DROP SCHEMA IF EXISTS funders CASCADE;

CREATE SCHEMA IF NOT EXISTS funders;

SET search_path TO funders,public;

CREATE TYPE payment_type AS ENUM('credit_card', 'bank_ach', 'paypal', 'bitcoin');

CREATE TYPE payment_state AS ENUM('success', 'failure', 'pending');

CREATE TABLE projects
(
    id SERIAL8 NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    goal NUMERIC NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    ship_date DATE NOT NULL,
    active BOOLEAN NOT NULL DEFAULT(true),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(end_date > start_date),
    CHECK(ship_date > end_date),
    CHECK(goal > 0)
);

ALTER SEQUENCE projects_id_seq INCREMENT BY 2 START WITH 31337 RESTART WITH 31337;

CREATE INDEX p_name_idx ON projects(name);

CREATE TABLE perks
(
    id SERIAL8 NOT NULL PRIMARY KEY,
    project_id INT8 NOT NULL REFERENCES projects (id),
    name VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    price NUMERIC NOT NULL,
    available INT8 NOT NULL,
    active BOOLEAN NOT NULL DEFAULT(true),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

ALTER SEQUENCE perks_id_seq INCREMENT BY 3 START WITH 31337 RESTART WITH 31337;

CREATE TABLE payments
(
    id UUID NOT NULL PRIMARY KEY,
    project_id INT8 NOT NULL REFERENCES projects (id),
    perk_id INT8 NOT NULL REFERENCES perks (id),
    payment_type PAYMENT_TYPE NOT NULL,
    amount NUMERIC NOT NULL,
    state PAYMENT_STATE NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE VIEW project_backers
AS
SELECT id,
       name,
       description,
       goal,
       CASE WHEN num_raised IS NULL THEN 0 ELSE num_raised END,
       CASE WHEN num_backers IS NULL THEN 0 ELSE num_backers END,
       start_date,
       end_date,
       ship_date,
       active
FROM projects
LEFT OUTER JOIN 
    (SELECT project_id, 
            sum(amount) AS num_raised,
            COUNT(1) AS num_backers
    FROM payments
    WHERE state = 'success'
    GROUP BY project_id) backers 
ON projects.id = backers.project_id 
ORDER BY id ASC;

CREATE VIEW perk_claims
AS
SELECT perks.id,
       perks.project_id,
       projects.name AS project_name,
       perks.name,
       perks.description,
       price,
       available,
       perks.active,
       CASE WHEN num_claimed IS NULL THEN 0 ELSE num_claimed END
FROM perks
INNER JOIN projects
ON perks.project_id = projects.id
LEFT OUTER JOIN
    (SELECT project_id,
            perk_id,
            COUNT(1) AS num_claimed
    FROM payments
    WHERE state = 'success'
    GROUP BY project_id, perk_id) claimed
ON perks.project_id = claimed.project_id
    AND perks.id = claimed.perk_id
ORDER BY project_id ASC;
