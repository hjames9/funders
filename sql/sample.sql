-- Projects

INSERT INTO funders.projects VALUES (DEFAULT, 'alpha', 'Alpha is the best consumer electronics product ever', 100000.50, '2016-09-01', '2016-11-01', '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.projects VALUES (DEFAULT, 'omega', 'Omega is the worst consumer electronics product ever', 50000, '2017-08-01', '2017-10-01', '2018-08-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.projects VALUES (DEFAULT, 'kappa', 'Kappa is an alright consumer electronics product', 75000, '2016-07-01', '2016-09-01', '2017-09-01', DEFAULT, current_timestamp, current_timestamp);

-- Perks

INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Alpha t-shirt', 'A black Alpha t-shirt', 25, 1000, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Green Alpha', 'A green version of Alpha', 500, 3000, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31339, 'Omega t-shirt', 'A black Omega t-shirt', 25, 1000, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31339, 'Purple Omega', 'A purple version of Omega', 500, 3000, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31341, 'Kappa t-shirt', 'A black Kappa t-shirt', 25, 1000, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31341, 'Gold Kappa', 'A gold version of Kappa', 500, 3000, DEFAULT, current_timestamp, current_timestamp);

-- Payments

INSERT INTO funders.payments VALUES ('3891fb08-f548-428c-80dc-e02f27ca2bdf', 31337, 31337, 'credit_card', 25, 'success', current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('0a245453-67fc-4ecb-a89a-f6e0ca393804', 31337, 31337, 'credit_card', 25, 'success', current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('5d5e0c4b-157f-467f-ae0d-bf699143a2f4', 31337, 31340, 'credit_card', 500, 'success', current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('74c2c75e-0f1a-4394-88a2-15b5cb298def', 31337, 31340, 'credit_card', 500, 'failure', current_timestamp, current_timestamp);

