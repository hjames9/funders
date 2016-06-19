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
    available_for_payment INT8 NOT NULL,
    available_for_pledge INT8 NOT NULL,
    ship_date DATE NOT NULL,
    active BOOLEAN NOT NULL DEFAULT(true),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(price > 0),
    CHECK(available_for_payment > 0 OR available_for_pledge > 0)
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
    CHECK(contact_email IS NULL OR contact_email ~* '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
    CHECK(amount > 0)
);

CREATE TABLE pledges
(
    id UUID NOT NULL PRIMARY KEY,
    campaign_id INT8 NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    perk_id INT8 NOT NULL REFERENCES perks (id) ON DELETE CASCADE,
    contact_email VARCHAR NULL,
    phone_number VARCHAR NULL,
    contact_opt_in BOOLEAN NOT NULL DEFAULT(true),
    amount NUMERIC NOT NULL,
    currency VARCHAR NOT NULL,
    advertise BOOLEAN NOT NULL DEFAULT(true),
    advertise_name VARCHAR NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CHECK(contact_email IS NULL OR contact_email ~* '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
    CHECK(contact_email IS NOT NULL OR phone_number IS NOT NULL),
    CHECK(advertise = FALSE OR (advertise = TRUE AND advertise_name IS NOT NULL))
);

CREATE VIEW campaign_backers
AS
SELECT id,
       name,
       description,
       goal,
       CASE WHEN amt_raised IS NULL THEN 0 ELSE amt_raised END,
       CASE WHEN num_backers IS NULL THEN 0 ELSE num_backers END,
       CASE WHEN amt_pledged IS NULL THEN 0 ELSE amt_pledged END,
       CASE WHEN num_pledgers IS NULL THEN 0 ELSE num_pledgers END,
       start_date,
       end_date,
       flexible,
       active
FROM campaigns
LEFT OUTER JOIN
    (SELECT campaign_id,
            sum(amount) AS amt_raised,
            COUNT(1) AS num_backers
    FROM payments
    WHERE state = 'success'
    GROUP BY campaign_id) backers
ON campaigns.id = backers.campaign_id
LEFT OUTER JOIN
    (SELECT campaign_id,
            sum(amount) AS amt_pledged,
            COUNT(1) AS num_pledgers
    FROM pledges
    GROUP BY campaign_id) pledgers
ON campaigns.id = pledgers.campaign_id
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
       available_for_payment,
       available_for_pledge,
       ship_date,
       CASE WHEN num_claimed IS NULL THEN 0 ELSE num_claimed END,
       CASE WHEN num_pledged IS NULL THEN 0 ELSE num_pledged END,
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
LEFT OUTER JOIN
    (SELECT campaign_id,
            perk_id,
            COUNT(1) AS num_pledged
    FROM pledges
    GROUP BY campaign_id, perk_id) pledged
ON perks.campaign_id = pledged.campaign_id
    AND perks.id = pledged.perk_id
ORDER BY campaign_id ASC;

CREATE VIEW active_payments
AS
SELECT
    payments.id,
    payments.campaign_id,
    payments.perk_id,
    campaigns.name AS campaign_name,
    perks.name AS perk_name,
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

CREATE VIEW active_pledges
AS
SELECT
    pledges.id,
    pledges.campaign_id,
    pledges.perk_id,
    campaigns.name AS campaign_name,
    perks.name AS perk_name,
    amount,
    pledges.currency,
    contact_email,
    phone_number,
    contact_opt_in,
    advertise,
    advertise_name
FROM pledges
INNER JOIN campaigns
ON pledges.campaign_id = campaigns.id
INNER JOIN perks
ON pledges.perk_id = perks.id
WHERE campaigns.active = TRUE AND perks.active = TRUE;

CREATE VIEW advertisements
AS
SELECT
    'payment' AS type,
    campaign_id,
    campaign_name,
    perk_id,
    active_payments.id AS payment_or_pledge_id,
    advertise,
    CASE WHEN advertise_other IS NULL THEN full_name ELSE advertise_other END AS advertise_name
FROM active_payments
INNER JOIN campaign_backers
ON active_payments.campaign_id = campaign_backers.id
WHERE active_payments.state = 'success'
UNION ALL
SELECT
    'pledge',
    campaign_id,
    campaign_name,
    perk_id,
    active_pledges.id,
    advertise,
    advertise_name
FROM active_pledges
INNER JOIN campaign_backers
ON active_pledges.campaign_id = campaign_backers.id;
