SET search_path TO funders,public;

COMMENT ON SCHEMA funders IS 'Funders schema holds all objects for application';

COMMENT ON TYPE account_type IS 'Enumeration for type of payment';
COMMENT ON TYPE payment_state IS 'Enumeration for state of payment';

-- Campaigns

COMMENT ON TABLE campaigns IS 'Campaigns table contains the available crowdfunding campaigns';

COMMENT ON COLUMN campaigns.id IS 'Primary key id of the campaigns table';
COMMENT ON COLUMN campaigns.name IS 'Name of the campaign';
COMMENT ON COLUMN campaigns.description IS 'Description of the campaign';
COMMENT ON COLUMN campaigns.goal IS 'Monetary goal of the campaign';
COMMENT ON COLUMN campaigns.start_date IS 'The starting date of the campaign';
COMMENT ON COLUMN campaigns.end_date IS 'The ending date of the campaign';
COMMENT ON COLUMN campaigns.flexible IS 'Flag if campaign is flexible or not.  Flexible is if campaign is all or none';
COMMENT ON COLUMN campaigns.active IS 'Flag for if campaign is active or not';
COMMENT ON COLUMN campaigns.created_at IS 'Timestamp of campaign creation';
COMMENT ON COLUMN campaigns.updated_at IS 'Timestamp of last time campaign was updated';

COMMENT ON CONSTRAINT campaigns_pkey ON campaigns IS 'Primary key constraint for campaigns id column';
COMMENT ON CONSTRAINT campaigns_check ON campaigns IS 'Check constraint used to enforce that the end date is after the start date';
COMMENT ON CONSTRAINT campaigns_goal_check ON campaigns IS 'Check constraint used to enforce that a given campaign goal is more than zero';
COMMENT ON INDEX c_name_idx IS 'B-tree index for name column for campaigns';

COMMENT ON SEQUENCE campaigns_id_seq IS 'Primary key sequence for campaigns table.  Values are obfuscated since they''re used on public interfaces';

-- Perks

COMMENT ON TABLE perks IS 'Perks table contains the perks for the crowdfunding campaigns';

COMMENT ON COLUMN perks.id IS 'Primary key id of the perks table';
COMMENT ON COLUMN perks.campaign_id IS 'Name of the perk';
COMMENT ON COLUMN perks.name IS 'Name of the perk';
COMMENT ON COLUMN perks.description IS 'Description of the perk';
COMMENT ON COLUMN perks.price IS 'Price of the perk';
COMMENT ON COLUMN perks.currency IS 'Currency of the perk';
COMMENT ON COLUMN perks.available IS 'Amount of available items for this perk';
COMMENT ON COLUMN perks.ship_date IS 'The shipping date of this perk';
COMMENT ON COLUMN perks.active IS 'Flag for if perk is active or not';
COMMENT ON COLUMN perks.created_at IS 'Timestamp of perk creation.';
COMMENT ON COLUMN perks.updated_at IS 'Timestamp of last time perk was updated';

COMMENT ON CONSTRAINT perks_pkey ON perks IS 'Primary key constraint for perks id column';
COMMENT ON CONSTRAINT perks_campaign_id_fkey ON perks IS 'Foreign key constraint for campaigns id column';
COMMENT ON CONSTRAINT perks_price_check ON perks IS 'Check constraint used to enforce that a price for a perk is more than zero';
COMMENT ON INDEX p_name_idx IS 'B-tree index for name column for perks';

COMMENT ON SEQUENCE perks_id_seq IS 'Primary key sequence for perks table.  Values are obfuscated since they''re used on public interfaces';

-- Payments

COMMENT ON TABLE payments IS 'Payments table contains all the payment transactions for the crowdfunding campaigns';

COMMENT ON COLUMN payments.id IS 'Primary key id of the payments table';
COMMENT ON COLUMN payments.campaign_id IS 'Reference to campaign that the payment is associated with';
COMMENT ON COLUMN payments.perk_id IS 'Reference to perk that the payment is associated with';
COMMENT ON COLUMN payments.account_type IS 'The type of method used for payment';
COMMENT ON COLUMN payments.name_on_payment IS 'The name of account owner';
COMMENT ON COLUMN payments.full_name IS 'Full name used for shipping';
COMMENT ON COLUMN payments.address1 IS 'Shipping address for perk';
COMMENT ON COLUMN payments.address2 IS 'Optional secondary address for perk';
COMMENT ON COLUMN payments.city IS 'Shipping city for perk';
COMMENT ON COLUMN payments.postal_code IS 'Shipping postal code for perk';
COMMENT ON COLUMN payments.country IS 'Shipping country for perk';
COMMENT ON COLUMN payments.amount IS 'Amount of the payment';
COMMENT ON COLUMN payments.currency IS 'Currency of the payment';
COMMENT ON COLUMN payments.state IS 'Current state of the payment';
COMMENT ON COLUMN payments.contact_email IS 'Contact e-mail of backer';
COMMENT ON COLUMN payments.contact_opt_in IS 'Flag if user wants to opt in for future mailings';
COMMENT ON COLUMN payments.advertise IS 'Whether to advertise user''s payment';
COMMENT ON COLUMN payments.advertise_other IS 'Use alternate value to advertise user''s payment';
COMMENT ON COLUMN payments.payment_processor_responses IS 'Transaction responses from payment processor';
COMMENT ON COLUMN payments.payment_processor_used IS 'Payment processor used to process this payment';
COMMENT ON COLUMN payments.created_at IS 'Timestamp of payment creation.';
COMMENT ON COLUMN payments.updated_at IS 'Timestamp of last time payment was updated';

COMMENT ON CONSTRAINT payments_pkey ON payments IS 'Primary key constraint for payments id column';
COMMENT ON CONSTRAINT payments_campaign_id_fkey ON payments IS 'Foreign key constraint for campaigns id column';
COMMENT ON CONSTRAINT payments_perk_id_fkey ON payments IS 'Foreign key constraint for perks id column';
COMMENT ON CONSTRAINT payments_contact_email_check ON payments IS 'Check constraint for payments table to make sure contact email is valid if provided';

-- Pledges

COMMENT ON TABLE pledges IS 'Pledges table contains all the pledged donations for the crowdfunding campaigns';

COMMENT ON COLUMN pledges.id IS 'Primary key id of the payments table';
COMMENT ON COLUMN pledges.campaign_id IS 'Reference to campaign that the pledge is associated with';
COMMENT ON COLUMN pledges.perk_id IS 'Reference to perk that the pledge is associated with';
COMMENT ON COLUMN pledges.contact_email IS 'Contact e-mail of pledger';
COMMENT ON COLUMN pledges.phone_number IS 'Phone number of pledger';
COMMENT ON COLUMN pledges.contact_opt_in IS 'Flag if user wants to opt in for future mailings';
COMMENT ON COLUMN pledges.amount IS 'Amount of the pledge';
COMMENT ON COLUMN pledges.currency IS 'Currency of the pledge';
COMMENT ON COLUMN pledges.advertise IS 'Whether to advertise user''s pledge';
COMMENT ON COLUMN pledges.advertise_name IS 'Name to advertise user''s pledge';
COMMENT ON COLUMN pledges.created_at IS 'Timestamp of pledge creation.';
COMMENT ON COLUMN pledges.updated_at IS 'Timestamp of last time pledge was updated';

COMMENT ON CONSTRAINT pledges_pkey ON pledges IS 'Primary key constraint for pledges id column';
COMMENT ON CONSTRAINT pledges_campaign_id_fkey ON pledges IS 'Foreign key constraint for campaigns id column';
COMMENT ON CONSTRAINT pledges_perk_id_fkey ON pledges IS 'Foreign key constraint for perks id column';
COMMENT ON CONSTRAINT pledges_contact_email_check ON pledges IS 'Check constraint for pledges table to make sure contact email is valid if provided';
COMMENT ON CONSTRAINT pledges_check ON pledges IS 'Check constraint for pledges table to make sure at least one of contact email or phone number is provided';
COMMENT ON CONSTRAINT pledges_check1 ON pledges IS 'Check constraint for pledges table to make sure an advertised name is provided if advertisement is requested';

-- Campaign backers

COMMENT ON VIEW campaign_backers IS 'Campaign backers is the campaigns table with aggregated data showing the amount of money raised and number of backers sourced from the payments table';

COMMENT ON RULE "_RETURN" ON campaign_backers IS 'Internal rule for campaign_backers view';

COMMENT ON COLUMN campaign_backers.id IS 'Primary key id of the campaigns table';
COMMENT ON COLUMN campaign_backers.name IS 'Name of campaigns table';
COMMENT ON COLUMN campaign_backers.description IS 'Description of campaigns table';
COMMENT ON COLUMN campaign_backers.goal IS 'Monetary goal of campaigns table';
COMMENT ON COLUMN campaign_backers.num_raised IS 'Amount of money raised in the campaign';
COMMENT ON COLUMN campaign_backers.num_backers IS 'Number of backers in the campaign';
COMMENT ON COLUMN campaign_backers.num_pledged IS 'Amount of money pledged in the campaign';
COMMENT ON COLUMN campaign_backers.num_pledgers IS 'Number of pledgers in the campaign';
COMMENT ON COLUMN campaign_backers.start_date IS 'Start date of the campaign';
COMMENT ON COLUMN campaign_backers.end_date IS 'End date of the campaign';
COMMENT ON COLUMN campaign_backers.flexible IS 'Flag if campaign is flexible or not.  Flexible is if campaign is all or none';
COMMENT ON COLUMN campaign_backers.active IS 'Flag if campaign is active or not';

-- Perk claims

COMMENT ON VIEW perk_claims IS 'Perk claims is the perks table with aggregated data with the number of items claimed sourced from the payments table';

COMMENT ON RULE "_RETURN" ON perk_claims IS 'Internal rule for perk_claims view';

COMMENT ON COLUMN perk_claims.id IS 'Primary key id of the perks table';
COMMENT ON COLUMN perk_claims.campaign_id IS 'Foreign key for the campaigns table';
COMMENT ON COLUMN perk_claims.campaign_name IS 'Name of the campaign associated with the perk';
COMMENT ON COLUMN perk_claims.name IS 'Name of the perk';
COMMENT ON COLUMN perk_claims.description IS 'Description of the perk';
COMMENT ON COLUMN perk_claims.price IS 'Price of the perk';
COMMENT ON COLUMN perk_claims.currency IS 'Currency of the perk';
COMMENT ON COLUMN perk_claims.available IS 'Amount of available items for the perk';
COMMENT ON COLUMN perk_claims.ship_date IS 'Ship date of the perk';
COMMENT ON COLUMN perk_claims.num_claimed IS 'Number of items claimed for the perk';
COMMENT ON COLUMN perk_claims.num_pledged IS 'Number of items pledged for the perk';
COMMENT ON COLUMN perk_claims.active IS 'Flag if perk is active or not';

-- Active payments

COMMENT ON VIEW active_payments IS 'Active payments is the payments table but from only active campaigns and perks';

COMMENT ON RULE "_RETURN" ON active_payments IS 'Internal rule for active_payments view';

COMMENT ON COLUMN active_payments.id IS 'Primary key id of the payments table';
COMMENT ON COLUMN active_payments.campaign_id IS 'Primary key id of the campaigns table';
COMMENT ON COLUMN active_payments.perk_id IS 'Primary key id of the perks table';
COMMENT ON COLUMN active_payments.campaign_name IS 'Name of campaign payment was made for';
COMMENT ON COLUMN active_payments.perk_name IS 'Name of perk payment was made for';
COMMENT ON COLUMN active_payments.account_type IS 'The type of method used for payment';
COMMENT ON COLUMN active_payments.name_on_payment IS 'The name of account owner';
COMMENT ON COLUMN active_payments.full_name IS 'Full name used for shipping';
COMMENT ON COLUMN active_payments.address1 IS 'Shipping address for perk';
COMMENT ON COLUMN active_payments.address2 IS 'Optional secondary address for perk';
COMMENT ON COLUMN active_payments.city IS 'Shipping city for perk';
COMMENT ON COLUMN active_payments.postal_code IS 'Shipping postal code for perk';
COMMENT ON COLUMN active_payments.country IS 'Shipping country for perk';
COMMENT ON COLUMN active_payments.amount IS 'Amount of the payment';
COMMENT ON COLUMN active_payments.currency IS 'Currency of the payment';
COMMENT ON COLUMN active_payments.state IS 'Current state of the payment';
COMMENT ON COLUMN active_payments.contact_email IS 'Contact e-mail of backer';
COMMENT ON COLUMN active_payments.contact_opt_in IS 'Flag if user wants to opt in for future mailings';
COMMENT ON COLUMN active_payments.advertise IS 'Whether to advertise user''s payment';
COMMENT ON COLUMN active_payments.advertise_other IS 'Use alternate value to advertise user''s payment';
COMMENT ON COLUMN active_payments.payment_processor_responses IS 'Transaction responses from payment processor';
COMMENT ON COLUMN active_payments.payment_processor_used IS 'Payment processor used to process this payment';

-- Active pledges

COMMENT ON VIEW active_pledges IS 'Active pledges is the pledges table but from only active campaigns and perks';

COMMENT ON RULE "_RETURN" ON active_pledges IS 'Internal rule for active_pledges view';

COMMENT ON COLUMN active_pledges.id IS 'Primary key id of the pledges table';
COMMENT ON COLUMN active_pledges.campaign_id IS 'Primary key id of the campaigns table';
COMMENT ON COLUMN active_pledges.perk_id IS 'Primary key id of the perks table';
COMMENT ON COLUMN active_pledges.campaign_name IS 'Name of campaign payment was made for';
COMMENT ON COLUMN active_pledges.perk_name IS 'Name of perk payment was made for';
COMMENT ON COLUMN active_pledges.amount IS 'Amount of the pledge';
COMMENT ON COLUMN active_pledges.currency IS 'Currency of the pledge';
COMMENT ON COLUMN active_pledges.contact_email IS 'Contact e-mail of pledger';
COMMENT ON COLUMN active_pledges.phone_number IS 'Contact phone number of pledger';
COMMENT ON COLUMN active_pledges.contact_opt_in IS 'Flag if user wants to opt in for future mailings';
COMMENT ON COLUMN active_pledges.advertise IS 'Whether to advertise user''s pledge';
COMMENT ON COLUMN active_pledges.advertise_name IS 'Use alternate value to advertise user''s pledge';

-- Advertisements

COMMENT ON VIEW advertisements IS 'Advertisements is the list of successful payments that would not mind advertising supporting the campaign.';

COMMENT ON RULE "_RETURN" ON advertisements IS 'Internal rule for advertisements view';

COMMENT ON COLUMN advertisements.type IS 'Advertisement is either from a payment or pledge';
COMMENT ON COLUMN advertisements.campaign_id IS 'Primary key id of the campaigns table';
COMMENT ON COLUMN advertisements.campaign_name IS 'Name of campaign payment or pledge was made for';
COMMENT ON COLUMN advertisements.perk_id IS 'Primary key id of the perks table';
COMMENT ON COLUMN advertisements.payment_or_pledge_id IS 'Primary key of payments or pledges table';
COMMENT ON COLUMN advertisements.advertise IS 'Determine if payment should be advertised';
COMMENT ON COLUMN advertisements.advertise_name IS 'Name of person that wants to advertise support';
