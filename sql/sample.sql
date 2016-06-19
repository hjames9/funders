-- Projects

INSERT INTO funders.campaigns VALUES (DEFAULT, 'alpha', 'Alpha is the best consumer electronics product ever', 10000, '2016-05-01', '2016-11-01', DEFAULT, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.campaigns VALUES (DEFAULT, 'omega', 'Omega is the worst consumer electronics product ever', 50000, '2017-08-01', '2017-10-01', DEFAULT, DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.campaigns VALUES (DEFAULT, 'kappa', 'Kappa is an alright consumer electronics product', 75000, '2016-07-01', '2016-09-01', DEFAULT, DEFAULT, current_timestamp, current_timestamp);

-- Perks

INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Alpha t-shirt', 'A black Alpha t-shirt', 25, 'USD', 1000, 2000, '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Green Alpha', 'A green version of Alpha', 500, 'USD', 5, 10, '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Silver Gamma', 'A silver version of Gamma', 500, 'USD', 3000, 6000, '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Orange Delta', 'An orange version of Delta', 500, 'USD', 3000, 6000, '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31337, 'Light blue Epsilon', 'A light blue version of Epsilon', 500, 'USD', 3000, 6000, '2017-09-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31339, 'Omega t-shirt', 'A black Omega t-shirt', 25, 'USD', 1000, 2000, '2018-08-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31339, 'Purple Omega', 'A purple version of Omega', 500, 'USD', 3000, 6000, '2018-08-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31341, 'Kappa t-shirt', 'A black Kappa t-shirt', 25, 'USD', 1000, 2000, '2017-07-01', DEFAULT, current_timestamp, current_timestamp);
INSERT INTO funders.perks VALUES (DEFAULT, 31341, 'Gold Kappa', 'A gold version of Kappa', 500, 'USD', 3000, 6000, '2017-07-01', DEFAULT, current_timestamp, current_timestamp);

-- Payments

INSERT INTO funders.payments VALUES ('3891fb08-f548-428c-80dc-e02f27ca2bdf', 31337, 31337, 'credit_card', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 25, 'USD', 'success', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('0a245453-67fc-4ecb-a89a-f6e0ca393804', 31337, 31337, 'credit_card', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 25, 'USD', 'success', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('5d5e0c4b-157f-467f-ae0d-bf699143a2f4', 31337, 31340, 'credit_card', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 500, 'USD', 'success', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('74c2c75e-0f1a-4394-88a2-15b5cb298def', 31337, 31340, 'credit_card', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 500, 'USD', 'failure', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('81e07c03-80a0-4d0e-a00c-ce764204da70', 31337, 31340, 'credit_card', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 500, 'USD', 'failure', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);
INSERT INTO funders.payments VALUES ('cef6fab5-ddb0-442e-ab10-b281dd982525', 31337, 31340, 'paypal', 'Donovan James', 'Donovan James', '12 East Tremont Avenue', DEFAULT, 'Bronx', '10467', 'USA', 500, 'USD', 'pending', DEFAULT, DEFAULT, DEFAULT, DEFAULT, DEFAULT, 'stripe', NULL, current_timestamp, current_timestamp);

-- Pledges

INSERT INTO funders.pledges VALUES ('8ae7e044-09f4-4e9e-981d-a6110e8bdc38', 31337, 31337, 'donovan.james@gmail.com', DEFAULT, TRUE, 25, 'USD', TRUE, 'Donovan James', current_timestamp, current_timestamp);
INSERT INTO funders.pledges VALUES ('5ccf7b22-6868-4583-8790-11898d9b51b8', 31337, 31337, 'rocman.offman@gmail.com', DEFAULT, TRUE, 25, 'USD', TRUE, 'Rocman Offman', current_timestamp, current_timestamp);
INSERT INTO funders.pledges VALUES ('592bec2a-14bd-4de8-8538-90c568b5770f', 31337, 31340, 'raul.ferris@gmail.com', DEFAULT, TRUE, 500, 'USD', FALSE, DEFAULT, current_timestamp, current_timestamp);
