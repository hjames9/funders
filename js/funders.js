/**
 * var funder = new Funder();
 *
 * funder.setUrl("https://host:port/<optional path>"); //Required
 * funder.setProjectsPath("/projects"); //Optional, default: "/projects"
 * funder.setPerksPath("/perks"); //Optional, default: "/perks"
 * funder.setPaymentsPath("/payments"); //Optional, default: "/payments"
 *
 * funder.getProject(params); //Synchronous
 * funder.getProject(params, //Asynchronous
 *      function(response, status, project) {}, //Success
 *      function(response, status, project) {}  //Error
 * );
 *
 * funder.getPerks(params); //Synchronous
 * funder.getPerks(params, //Asynchronous
 *      function(response, status, perks) {}, //Success
 *      function(response, status, perks) {}  //Error
 * );
 *
 * funder.makePayment(params); //Synchronous
 * funder.makePayment(params, //Asynchronous
 *      function(response, status, payment) {}, //Success
 *      function(response, status, payment) {}  //Error
 * );
 *
 */

function isBetween(value, min, max)
{
    return(value >= min && value <= max);
};

function isInformational(code)
{
    return isBetween(code, 100, 199);
};

function isSuccess(code)
{
    return isBetween(code, 200, 299);
};

function isRedirection(code)
{
    return isBetween(code, 300, 399);
};

function isClientError(code)
{
    return isBetween(code, 400, 499);
};

function isServerError(code)
{
    return isBetween(code, 500, 599);
};

function isError(code)
{
    return isClientError(code) || isServerError(code);
};

function useMethodBody(method)
{
    switch(method)
    {   
    case "GET":
    case "HEAD":
    case "OPTIONS":
        return false;
    case "POST":
    case "PUT":
    case "PATCH":
    case "DELETE":
    default:
        return true;
    }   
};

function convertJSONToURLEncodedString(json)
{
    var pairs = [];
    for(var key in json)
    {
        if(json.hasOwnProperty(key))
            pairs.push(encodeURIComponent(key) + "=" + encodeURIComponent(json[key]));
    }

    return pairs.join("&");
};

function Funder()
{
};

Funder.prototype.setUrl = function(url) {
    this.url = url;
};

Funder.prototype.getUrl = function() {
    return this.url;
}

Funder.prototype.setProjectsPath = function(projectsPath) {
    this.projectsPath = projectsPath;
};

Funder.prototype.getProjectsPath = function() {
    return this.projectsPath;
};

Funder.prototype.setPerksPath = function(perksPath) {
    this.perksPath = perksPath;
};

Funder.prototype.getPerksPath = function() {
    return this.perksPath;
};

Funder.prototype.setPaymentsPath = function(paymentsPath) {
    this.paymentsPath = paymentsPath;
};

Funder.prototype.getPaymentsPath = function() {
    return this.paymentsPath;
};

Funder.prototype.getProject = function(params, successFunc, errorFunc) {
    return this.internalRequest(params, successFunc, errorFunc, this.getUrl() + this.getProjectsPath(), "GET");
};

Funder.prototype.getPerks = function(params, successFunc, errorFunc) {
    return this.internalRequest(params, successFunc, errorFunc, this.getUrl() + this.getPerksPath(), "GET");
};

Funder.prototype.makePayment = function(params, successFunc, errorFunc) {
    return this.internalRequest(params, successFunc, errorFunc, this.getUrl() + this.getPaymentsPath(), "POST");
};

Funder.prototype.internalRequest = function(params, successFunc, errorFunc, url, method) {
    var xmlHttp = new XMLHttpRequest();
    var async = (null != successFunc || null != errorFunc);
    var that = this;

    xmlHttp.onload = function(e)
    {
        if(null != successFunc && isSuccess(xmlHttp.status)) {
            successFunc(JSON.parse(xmlHttp.responseText), xmlHttp.status, that);
        } else if(null != errorFunc && isError(xmlHttp.status)) {
            errorFunc(JSON.parse(xmlHttp.responseText), xmlHttp.status, that);
        }
    };

    xmlHttp.onerror = function(e)
    {
        if(null != errorFunc) {
            if(0 != xmlHttp.status) {
                errorFunc(JSON.parse(xmlHttp.responseText), xmlHttp.status, that);
            } else {
                //Titanium handles connection down errors here.  Browsers typically throw an exception on send
                var error = { "code":503,
                              "code_message":"Service unavailable",
                              "message":e.error
                            };

                errorFunc(error, error.code, that);
            }
        }
    };

    try
    {
        var methodBody = useMethodBody(method);
        var requestStr = convertJSONToURLEncodedString(params);

        if(methodBody)
        {
            xmlHttp.open(method, url, async);
            xmlHttp.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xmlHttp.send(requestStr);
        }
        else
        {
            xmlHttp.open(method, url + "?" + requestStr, async);
            xmlHttp.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xmlHttp.send();
        }

        if(!async) {
            if(0 != xmlHttp.status) {
                return JSON.parse(xmlHttp.responseText);
            } else {
                var error = { "code":503,
                              "code_message":"Service unavailable",
                              "message":exp.name
                            };

                return error;
            }
        }
    }
    catch(exp)
    {
        //Browsers typically handle connection refused errors by throwing an exception on send
        var error = { "code":503,
                      "code_message":"Service unavailable",
                      "message":exp.name
                    };

        if(!async) {
            return error;
        } else {
            if(null != errorFunc) {
                errorFunc(error, error.code, that);
            }
        }
    }
};
