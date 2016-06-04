DROP SCHEMA IF EXISTS funders CASCADE;

CREATE SCHEMA IF NOT EXISTS funders;

SET search_path TO funders,public;

CREATE TYPE account_type AS ENUM('credit_card', 'paypal', 'bitcoin');

CREATE TYPE payment_state AS ENUM('success', 'failure', 'pending');

CREATE TABLE campaigns
(
    id SERIAL8 NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    goal NUMERIC NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    flexible BOOLEAN NOT NULL DEFAULT(false),
    active BOOLEAN NOT NULL DEFAULT(true),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(end_date > start_date),
    CHECK(goal > 0)
);

ALTER SEQUENCE campaigns_id_seq INCREMENT BY 2 START WITH 31337 RESTART WITH 31337;

CREATE UNIQUE INDEX c_name_idx ON campaigns(name);

CREATE TABLE perks
(
    id SERIAL8 NOT NULL PRIMARY KEY,
    campaign_id INT8 NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    price NUMERIC NOT NULL,
    currency VARCHAR NOT NULL,
    available INT8 NOT NULL,
    ship_date DATE NOT NULL,
    active BOOLEAN NOT NULL DEFAULT(true),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(price > 0)
);

ALTER SEQUENCE perks_id_seq INCREMENT BY 3 START WITH 31337 RESTART WITH 31337;

CREATE UNIQUE INDEX p_name_idx ON perks(name, campaign_id);

CREATE TABLE payments
(
    id UUID NOT NULL PRIMARY KEY,
    campaign_id INT8 NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    perk_id INT8 NOT NULL REFERENCES perks (id) ON DELETE CASCADE,
    account_type ACCOUNT_TYPE NOT NULL,
    name_on_payment VARCHAR NOT NULL,
    full_name VARCHAR NOT NULL,
    address1 VARCHAR NOT NULL,
    address2 VARCHAR NULL,
    city VARCHAR NOT NULL,
    postal_code VARCHAR NOT NULL,
    country VARCHAR NOT NULL,
    amount NUMERIC NOT NULL,
    currency VARCHAR NOT NULL,
    state PAYMENT_STATE NOT NULL,
    contact_email VARCHAR NULL,
    contact_opt_in BOOLEAN NOT NULL DEFAULT(true),
    advertise BOOLEAN NOT NULL DEFAULT(true),
    advertise_other VARCHAR NULL,
    payment_processor_responses JSONB[] NULL,
    payment_processor_used VARCHAR NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(contact_email IS NULL OR contact_email ~* '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$')
);

CREATE VIEW campaign_backers
AS
SELECT id,
       name,
       description,
       goal,
       CASE WHEN num_raised IS NULL THEN 0 ELSE num_raised END,
       CASE WHEN num_backers IS NULL THEN 0 ELSE num_backers END,
       start_date,
       end_date,
       flexible,
       active
FROM campaigns
LEFT OUTER JOIN 
    (SELECT campaign_id,
            sum(amount) AS num_raised,
            COUNT(1) AS num_backers
    FROM payments
    WHERE state = 'success'
    GROUP BY campaign_id) backers
ON campaigns.id = backers.campaign_id
ORDER BY id ASC;

CREATE VIEW perk_claims
AS
SELECT perks.id,
       perks.campaign_id,
       campaigns.name AS campaign_name,
       perks.name,
       perks.description,
       price,
       currency,
       available,
       ship_date,
       CASE WHEN num_claimed IS NULL THEN 0 ELSE num_claimed END,
       perks.active
FROM perks
INNER JOIN campaigns
ON perks.campaign_id = campaigns.id
LEFT OUTER JOIN
    (SELECT campaign_id,
            perk_id,
            COUNT(1) AS num_claimed
    FROM payments
    WHERE state = 'success'
    GROUP BY campaign_id, perk_id) claimed
ON perks.campaign_id = claimed.campaign_id
    AND perks.id = claimed.perk_id
ORDER BY campaign_id ASC;

CREATE VIEW active_payments
AS
SELECT
    payments.id,
    payments.campaign_id,
    payments.perk_id,
    account_type,
    name_on_payment,
    full_name,
    address1,
    address2,
    city,
    postal_code,
    country,
    amount,
    payments.currency,
    state,
    contact_email,
    contact_opt_in,
    advertise,
    advertise_other,
    payment_processor_responses,
    payment_processor_used
FROM payments
INNER JOIN campaigns
ON payments.campaign_id = campaigns.id
INNER JOIN perks
ON payments.perk_id = perks.id
WHERE campaigns.active = TRUE AND perks.active = TRUE;
