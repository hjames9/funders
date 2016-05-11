# Funders:  Toolkit for creating your own crowdfunding campaign 
A Go-based http server to create your own crowdfunding campaign on your website.  Crowdfunding data is persisted in a Postgresql database and payment integration currently supports Stripe.

## funders - http server

### Setup - Set environmental variables
    DATABASE_URL=postgres://user:password@localhost:5432/funder_db (no default)
    DB_USER=hjames (no default, ignored with DATABASE_URL set)
    DB_PASSWORD=blahblah (no default, ignored with DATABASE_URL set)
    DB_NAME=funder_db (no default, ignored with DATABASE_URL set)
    DB_HOST=localhost (default is localhost, ignored with DATABASE_URL set)
    DB_PORT=5432 (default is 5432, ignored with DATABASE_URL set)
    DB_MAX_OPEN_CONNS=100 (default is 10)
    DB_MAX_IDLE_CONNS=100 (default is 0)
    PGAPPNAME=funders (default is funders)
    SSL_REDIRECT=true (default is false)
    HOST=localhost (default is all interfaces (blank))
    PORT=8080 (default is 3000)
    MARTINI_ENV=production (default is development)
    ALLOW_HEADERS=X-Requested-With,X-Forwarded-For (default is empty for only default headers)
    BOTDETECT_FIELDLOCATION=body (default is body, can be body or header)
    BOTDETECT_FIELDNAME=middlename (default is spambot)
    BOTDETECT_FIELDVALUE=iamhuman (default is blank)
    BOTDETECT_MUSTMATCH=true (default is true)
    BOTDETECT_PLAYCOY=true (default is true)
    ASYNC_REQUEST=true (default is false)
    ASYNC_REQUEST_SIZE=100000 (default is 100000)
    ASYNC_PROCESS_INTERVAL=10 (default is 5 seconds)
    IP_ADDRESS_LOCATION=xff_first (default is normal, can be normal, xff_first, xff_last)
    STRING_SIZE_LIMIT=1000 (default is 500)