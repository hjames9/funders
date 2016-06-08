function buyPerk(event)
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");
    funder.addAdhocField("currency", "USD");

    var perkId = event.target.id.substring(0, event.target.id.length - 1);

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

    payment = funder.makePayment(paymentParams);

    if(isSuccess(payment.Code)) {
        setTimeout(function() {
            $("#purchaseStatus").text("Success buying perk: " + payment.Message);
            loadFunders();
        }, 6000);
    } else {
        $("#purchaseStatus").text("Error buying perk: " + payment.Message);
    }
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

    pledge = funder.makePledge(pledgeParams);

    if(isSuccess(pledge.Code)) {
        setTimeout(function() {
            $("#purchaseStatus").text("Success pledging perk: " + pledge.Message);
            loadFunders();
        }, 6000);
    } else {
        $("#purchaseStatus").text("Error pledging perk: " + pledge.Message);
    }
};

function loadFunders()
{
    var funder = new Funder();
    funder.setUrl("http://localhost:3000");

    var campaignParams = { "name" : "alpha" };
    campaign = funder.getCampaign(campaignParams);

    var progressBar = $("#progressbar");
    progressBar.progressbar({max: campaign.goal, value: campaign.numRaised});
    $('#campaignName').text(campaign.name);
    $('#numBackers').text(campaign.numBackers);
    $('#numRaised').text(campaign.numRaised);
    $('#numPledgers').text(campaign.numPledgers);
    $('#numPledged').text(campaign.numPledged);
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
        $('#perks').append('<tr><td>' + value.name + '</td><td>' + value.description + '</td><td>' + value.price + '</td><td>' + value.numClaimed + '</td><td>' + value.numPledged + '</td><td>' + value.available + '</td><td>' + value.shipDate + '</td><td><button id="' + value.id + 'b">Buy perk</button></td>' + '<td><button id="' + value.id + 'p">Pledge perk</button></td></tr>');
        $("#" + value.id + "b").click(buyPerk);
        $("#" + value.id + "p").click(pledgePerk);
    });

    $('#advertisements').children('li').each(function() {
        $(this).remove();
    });

    var advertisementsParams = { "campaign_name" : "alpha" };
    advertisements = funder.getAdvertisements(advertisementsParams);

    $.each(advertisements, function(index, value) {
        $('#advertisements').append('<li>' + value.advertiseName + '</li>');
    });
};

loadFunders();
