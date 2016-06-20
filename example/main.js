function buyPerkCreditCard(event)
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");
    funder.addAdhocField("currency", "USD");

    var perkId = event.target.id.substring(0, event.target.id.length - 2);

    var paymentParams = { "campaignId" : "31337",
                          "perkId" : perkId,
                          "accountType" : "credit_card",
                          "nameOnPayment" : "John Doe",
                          //"creditCardAccountNumber" : "4000000000000002", //Reject number
                          "creditCardAccountNumber" : "5555555555554444",  //Accept number
                          "creditCardExpirationDate" : "2019-09-01",
                          "creditCardCvv" : "444",
                          "creditCardPostalCode" : "10467",
                          "fullName" : "John Doe",
                          "address1" : "55555 White Plains Road",
                          "address2" : "Apt. 555",
                          "city" : "Bronx",
                          "postalCode" : "10467",
                          "country" : "US",
                          "advertise" : true,
                          "advertiseOther" : "Philly Bronx"
                        };

    if(perkId == 31337)
    {
        console.log("Adding pledge id to payment");
        paymentParams["pledgeId"] = "8ae7e044-09f4-4e9e-981d-a6110e8bdc38";
    }

    successFunc = function(response, status, fun)
    {
        function reload()
        {
            $("#purchaseStatus").text("Success buying perk with credit card: " + response.perk.name);
            loadFunders();
        };

        if(status == 202) {
            setTimeout(reload, 6000);
        } else {
            reload();
        }
    };

    errorFunc = function(response, status, fun)
    {
        $("#purchaseStatus").text("Error buying perk with credit card: " + response.Message);
    };

    funder.makePayment(paymentParams, successFunc, errorFunc);
};

function buyPerkPaypal(event)
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");
    funder.addAdhocField("currency", "USD");

    var perkId = event.target.id.substring(0, event.target.id.length - 2);

    var paymentParams = { "campaignId" : "31337",
                          "perkId" : perkId,
                          "accountType" : "paypal",
                          "nameOnPayment" : "John Doe",
                          "paypalRedirectUrl" : "https://originallocation.com?redirect=true",
                          "paypalCancelUrl" : "https://originallocation.com?cancel=true",
                          "fullName" : "John Doe",
                          "address1" : "55555 White Plains Road",
                          "address2" : "Apt. 555",
                          "city" : "Bronx",
                          "postalCode" : "10467",
                          "country" : "US",
                          "advertise" : true,
                          "advertiseOther" : "Philly Bronx"
                        };

    successFunc = function(response, status, fun)
    {
        function reload()
        {
            //Update payment
            var updatePaymentParams = { "id" : response.id,
                                        "campaignId" : "31337",
                                        "perkId" : perkId,
                                        "accountType" : "paypal",
                                        "nameOnPayment" : "John Doe",
                                        "paypalPayerId" : "badpayerid",
                                        "paypalPaymentId" : "badpaymentid",
                                        "paypalToken" : "badtoken"
                                      };

            successPaypalFunc = function(response2, status2, fun2)
            {
                $("#purchaseStatus").text("Success buying perk with paypal: " + response2.perk.name);
                loadFunders();
            };

            errorPaypalFunc = function(response2, status2, fun2)
            {
                $("#purchaseStatus").text("Error buying perk with paypal: " + response2.Message);
            };

            funder.updatePayment(updatePaymentParams, successPaypalFunc, errorPaypalFunc);

        };

        if(status == 202) {
            setTimeout(reload, 6000);
        } else {
            reload();
        }
    };

    errorFunc = function(response, status, fun)
    {
        $("#purchaseStatus").text("Error buying perk with paypal: " + response.Message);
    };

    funder.makePayment(paymentParams, successFunc, errorFunc);
};

function pledgePerk(event)
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");
    funder.addAdhocField("currency", "USD");

    var perkId = event.target.id.substring(0, event.target.id.length - 1);

    var pledgeParams = { "campaignId" : "31337",
                         "perkId" : perkId,
                         "contactEmail" : "elmer.fudd@gmail.com",
                         "phoneNumber" : "555-718-2122",
                         "advertise" : true,
                         "advertiseName" : "Philly Queens"
                       };

    successFunc = function(response, status, fun)
    {
        function reload()
        {
            $("#purchaseStatus").text("Success pledging perk: " + response.perk.name);
            loadFunders();
        };

        if(status == 202) {
            setTimeout(reload, 6000);
        } else {
            reload();
        }
    };

    errorFunc = function(response, status, fun)
    {
        $("#purchaseStatus").text("Error pledging perk: " + response.Message);
    };

    funder.makePledge(pledgeParams, successFunc, errorFunc);
};

function loadFunders()
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");

    var campaignParams = { "name" : "alpha" };
    campaign = funder.getCampaign(campaignParams);

    var progressBar = $("#progressbar");
    progressBar.progressbar({max: campaign.goal, value: campaign.amtRaised});
    $('#campaignName').text(campaign.name);
    $('#numBackers').text(campaign.numBackers);
    $('#amtRaised').text(campaign.amtRaised);
    $('#numPledgers').text(campaign.numPledgers);
    $('#amtPledged').text(campaign.amtPledged);
    $('#goal').text(campaign.goal);
    $('#startDate').text(campaign.startDate);
    $('#endDate').text(campaign.endDate);

    var perksParams = { "campaign_name" : "alpha" };
    perks = funder.getPerks(perksParams);

    $('#perks').children('tbody').children('tr').each(function() {
        if(!$(this).hasClass('perksHeader')) {
            $(this).remove();
        }
    });

    $.each(perks, function(index, value) {
        $('#perks').append('<tr><td>' + value.name + '</td><td>' + value.description + '</td><td>' + value.price + '</td><td>' + value.numClaimed + '</td><td>' + value.numPledged + '</td><td>' + value.availableForPayment + '</td><td>' + value.availableForPledge + '</td><td>' + value.shipDate + '</td><td><button id="' + value.id + 'bc">Buy perk</button></td>' + '<td><button id="' + value.id + 'bp">Buy perk</button></td>' + '<td><button id="' + value.id + 'p">Pledge perk</button></td></tr>');
        $("#" + value.id + "bc").click(buyPerkCreditCard);
        $("#" + value.id + "bp").click(buyPerkPaypal);
        $("#" + value.id + "p").click(pledgePerk);
    });

    $('#advertisements').children('li').each(function() {
        $(this).remove();
    });

    var advertisementsParams = { "campaign_name" : "alpha" };
    advertisements = funder.getAdvertisements(advertisementsParams);

    $.each(advertisements, function(index, value) {
        $('#advertisements').append('<li>' + 'Type: ' + value.type + ' for ' + value.advertiseName + '</li>');
    });
};

loadFunders();
