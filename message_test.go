package smtpd_test

import (
	"mime"
	"strings"
	"testing"

	"net/mail"

	"github.com/mailsac/smtpd"
)

const (
	plainHTMLEmail = `From: Sender <sender@example.com>
Date: Mon, 16 Jan 2017 16:59:33 -0500
Subject: Multipart Message
MIME-Version: 1.0
Content-Type: text/html
To: recipient1@example.com, "Recipient 2" <recipient2@example.com>
Message-ID: <examplemessage@example.com>
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>=F0=9F=90=9D
  </body>
</html>`

	alternativeEmail = `From: Sender <sender@example.com>
Date: Mon, 16 Jan 2017 16:59:33 -0500
Subject: Multipart Message
MIME-Version: 1.0
Content-Type: multipart/alternative;
 	 boundary="_=test=_bbd1e98aa6c34ef59d8d102a0e795027"
To: recipient1@example.com, "Recipient 2" <recipient2@example.com>
Message-ID: <examplemessage@example.com>

--_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: text/plain; charset="UTF-8"
Content-Transfer-Encoding: quoted-printable

Sending bees

=F0=9F=90=9D

--_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: text/html; charset="UTF-8"
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>=F0=9F=90=9D
  </body>
</html>

--_=test=_bbd1e98aa6c34ef59d8d102a0e795027--
`
	emailWithAttachment = `From: Sender <sender@example.com>
Date: Mon, 16 Jan 2017 16:59:33 -0500
Subject: Multipart Message
MIME-Version: 1.0
Content-Type: multipart/mixed;
 	 boundary="_=test=_bbd1e98aa6c34ef59d8d102a0e795027"
To: recipient1@example.com, "Recipient 2" <recipient2@example.com>
Message-ID: <examplemessage@example.com>

--_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: multipart/alternative; boundary="_=ALT_=test=_bbd1e98aa6c34ef59d8d102a0e795027"

--_=ALT_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: text/plain; charset="UTF-8"
Content-Transfer-Encoding: quoted-printable

Sending bees

=F0=9F=90=9D

--_=ALT_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: text/html; charset="UTF-8"
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>=F0=9F=90=9D
  </body>
</html>

--_=ALT_=test=_bbd1e98aa6c34ef59d8d102a0e795027--
--_=test=_bbd1e98aa6c34ef59d8d102a0e795027
Content-Type: text/calendar; name="invite.ics"
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="invite.ics"

QkVHSU46VkNBTEVOREFSClZFUlNJT046Mi4wClBST0RJRDotLy9tYWlscHJvdG8vL01haWxQcm90bwpDQUxTQ0FMRTpHUkVHT1JJQU4KQkVHSU46VkVWRU5UCkRUU1RBTVA6MjAxNzAxMTZUMTU0MDAwClVJRDpteWNvb2xldmVudEBtYWlscHJvdG8KCkRUU1RBUlQ7VFpJRD0iQW1lcmljYS9OZXdfWW9yayI6MjAxNzAxMThUMTEwMDAwCkRURU5EO1RaSUQ9IkFtZXJpY2EvTmV3X1lvcmsiOjIwMTcwMTE4VDEyMDAwMApTVU1NQVJZOlNlbmQgYW4gZW1haWwKTE9DQVRJT046VGVzdApFTkQ6VkVWRU5UCkVORDpWQ0FMRU5EQVI=
--_=test=_bbd1e98aa6c34ef59d8d102a0e795027--`

	utf8EncodedFromName = `From: Sender \u0014\<sender@example.com>
Date: Mon, 16 Jan 2017 16:59:33 -0500
Subject: Multipart Message
MIME-Version: 1.0
Content-Type: text/html
To: recipient1@example.com, "Recipient 2" <recipient2@example.com>
Message-ID: <examplemessage@example.com>
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>=F0=9F=90=9D
  </body>
</html>`

	emailWithInvalidBody = `From: Sender <sender@example.com>
Date: Mon, 16 Jan 2017 16:59:33 -0500
Subject: Invalid Body Message
MIME-Version: 1.0
Content-Type: text/html
To: recipient1@example.com, "Recipient 2" <recipient2@example.com>
Message-ID: <examplemessage@example.com>
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>=FG=XX==
  </body>
</html>`

	emailWithNoBody = `ARC-Seal: i=1; a=rsa-sha256; s=arcselector9901; d=microsoft.com; cv=none;
 b=BKJppuHSvxfkfpPTnFjsbREppvyanDeEU5HBw6ukRdGEZdipk9DsnNtulC/AZkzH/X44GTas3MG/cE8NJ9tQFMAgxvyQvdEBSMJ+VMzBzCpE1F02xhO1/brn6NkViZK9s/YsL2QBlMG5neKvk4grdtdMCGwzAkipjC3ffRlpeWi36Hnji75qgk8PLoWgZltMlGiKnYIny2DhBF4xfsmQ5yY3rGHwQICn1mN8QY0jfcGopwIg4Ldo7IfZetaEaLiDRrtvj9vZCwdfe8fb+fV3s2viFJa4kPHstYviLsRlcbUPh1vUvuQMkzvCri6C2FW6+NH/b9TZsU6PFsaTksHTcg==
ARC-Message-Signature: i=1; a=rsa-sha256; c=relaxed/relaxed; d=microsoft.com;
 s=arcselector9901;
 h=From:Date:Subject:Message-ID:Content-Type:MIME-Version:X-MS-Exchange-AntiSpam-MessageData-ChunkCount:X-MS-Exchange-AntiSpam-MessageData-0:X-MS-Exchange-AntiSpam-MessageData-1;
 bh=47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=;
 b=fnpe8oImTQnK410gPqImvZCD1FLpA5mNtYBKm99VqOUXwj7Dxsb2A5HbInR+mjLHoCnj+QYrUo0xQhbip7KD+llcj18msvdcyc+ufJhTW0F1KLRTN3PoiCyo4KMZInXNAeATP7ON0joBCrSfbr6mcI/Z2oBM6wvk4sFq9HEjlRCmSsOOzLoOAzYSK8vbquxfqZ5Z22vaABKNRtVr6FCBFD8NqwKRz/Rf5TiVoM/sGkmOD32ZGkjxn186ob8qIuCggXh9U54G4eOLYwEA0uhBvTYDdX/YDFWfMxcQAo4wXAi1b9tL4pOZl2DrRIUj2H0oMqM3SljR6M6BG8bOOQH8tA==
ARC-Authentication-Results: i=1; mx.microsoft.com 1; spf=pass
 smtp.mailfrom=forkingsoftware.com; dmarc=pass action=none
 header.from=forkingsoftware.com; dkim=pass header.d=forkingsoftware.com;
 arc=none
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=forkingsoftware.com;
 s=selector1;
 h=From:Date:Subject:Message-ID:Content-Type:MIME-Version:X-MS-Exchange-SenderADCheck;
 bh=47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=;
 b=DwmpFrTaIe1qSbAzdEOsKgbPRXRYrbKDSp6FEqKHvDzoUoZVln3oADENxEqEFbvTPHDuKQoC4fJB/7etZaTkgtVYFNhbOEYYwsg2qIFoP1HaekuCwqkjI6t1ohoghjNBjNUZzHnnGdF4YkZrdRuhh2Twij4Px3xpb2cCNmHYLbZJEdiBQJFUjlzUkjWMEiaaOGfkgPOJNeXor6d7hyRvXGumrhs7zRN+L3VoZlds8Qwin67mnFenLj77S9ukVCs9KiVmRhqOAbL6HkVAZXF4ccQ0bXTFX2Ip9dcJHtc3MW7pV6glmhmVD0LMM6OvS6/H+tsefv4j4Gtn6stqGiChVw==
Received: from SJ0PR18MB4899.namprd18.prod.outlook.com (2603:10b6:a03:40a::11)
 by MN2PR18MB3421.namprd18.prod.outlook.com (2603:10b6:208:16b::23) with
 Microsoft SMTP Server (version=TLS1_2,
 cipher=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384) id 15.20.5373.17; Fri, 24 Jun
 2022 17:29:08 +0000
Received: from SJ0PR18MB4899.namprd18.prod.outlook.com
 ([fe80::815:c441:140b:90e]) by SJ0PR18MB4899.namprd18.prod.outlook.com
 ([fe80::815:c441:140b:90e%6]) with mapi id 15.20.5373.015; Fri, 24 Jun 2022
 17:29:08 +0000
From: Forking Team <team@forkingsoftware.com>
To: "asdf@mailsac-staging.com" <asdf@mailsac-staging.com>
Subject:
Thread-Index: AQHYh+/pLgiR3cDgnkCfp6Yvx97a9w==
Date: Fri, 24 Jun 2022 17:29:08 +0000
Message-ID: <4980494D-E7BC-4B37-BAB8-B6EF12280480@forkingsoftware.com>
Accept-Language: en-US
Content-Language: en-US
X-MS-Has-Attach:
X-MS-TNEF-Correlator:
authentication-results: dkim=none (message not signed)
 header.d=none;dmarc=none action=none header.from=forkingsoftware.com;
x-ms-publictraffictype: Email
x-ms-office365-filtering-correlation-id: f9dc7077-113f-459b-9fc2-08da56070b9c
x-ms-traffictypediagnostic: MN2PR18MB3421:EE_
x-ms-exchange-senderadcheck: 1
x-ms-exchange-antispam-relay: 0
x-microsoft-antispam: BCL:0;
x-microsoft-antispam-message-info: /4EY81sMb5EYL0eYKnfTP+WKyDG3le9p5ARkEvsOZwnmIJb+JVmqIK3XKWoNo9FskEFRAowXGAa0KLJAuqbJdPrfOHDfvmZvM8mIrlPm7vkoBFW+hMSzkrgvkKDfN4ny2RTV2IQSZWQyeVE7sjcSL72IZaF1mTpin0saLQUIi29M6VWBkLcI2iUmA5IvHveHQo+PdDpxdv1InoEA4/j178azOMcH/sOCFAB3Se3e/FboeJsN1KRiDjNqG0bY7uXL8JqHwhkDLee0u4FpX50hBUopwz4xFgEsQ0UuD1kJEbOaMUumVR1r/11HLywxX6DQ/1iloZVh/n5s49Y2c1MV8kwFXwzMhwBHH8cDVjLzLgUngOdukNfLpXi4rjII3No9zsc4Bry+WSqVYEknIF3oWIhaKbTxNAalliE8VXotJ+b9vtqR40yC424yfm7bKOkKFRjsrePr9Z189bXaJtL+IyR/fAfLgNqsMK7FuGSHERHQlOwHfjUiPMzWzHIcICbDqMurB1m3jO7vKX99LRhFJb22zI2Z4eZWx9Te5z6RVqW4C1uqKbAkmwnJSUuJCK31
x-forefront-antispam-report: CIP:255.255.255.255;CTRY:;LANG:en;SCL:1;SRV:;IPV:NLI;SFV:NSPM;H:SJ0PR18MB4899.namprd18.prod.outlook.com;PTR:;CAT:NONE;SFS:(13230016)(136003)(39830400003)(346002)(396003)(366004)(376002)(309900001)(66476007)(64756008)(6916009)(66556008)(66446008)(91956017)(76116006)(316002)(66946007)(38100700002)(5406001)(6512007)(4270600006)(186003)(621065003)(73894004)(86362001)(33656002)(2616005)(122000001)(6486002)(71200400001)(6506007)(38070700005)(41300700001)(36756003)(2906002)(8936002)(478600001)(26005)(45980500001);DIR:OUT;SFP:1102;
x-ms-exchange-antispam-messagedata-chunkcount: 1
x-ms-exchange-antispam-messagedata-0: =?iso-8859-1?Q?0I3uyNrifnFqAy4GPlLGnDHFVJZlgUjHsVsU2zOVH9e9hK0FDVfUCHZc22?=
 =?iso-8859-1?Q?Alxc2D0rflmiw3UWszwt69G6M0IU2JooR9xPuiWzMTMnkxMZrrKGaymQpv?=
 =?iso-8859-1?Q?YFhrzWvwegryfLePnplgeS7BSqpJIEfGG/NB95qRpYSIET3XH6Gk/DjC8w?=
 =?iso-8859-1?Q?kpVwr3MXIlPJuIt6cBWvzORR9MgFEi5L3eE3EYyj/t89+OlRpxp4DaDHiu?=
 =?iso-8859-1?Q?bVpoOiEykrL6yaBoYya15lMs4cc95OwMGx/VHho2TU6aOD3HvmLWWJnhTv?=
 =?iso-8859-1?Q?lripi0f1dm4Kk0LHaJZWm4KMUr1m9cGX3JTNmx8o4Dxm6X7JI4UWPbeJG8?=
 =?iso-8859-1?Q?WG2bER6ZS83TJ0sLKj9ys5Cf2/6h0m1seC0amMfhlhz0jVbbznPSSn2n9m?=
 =?iso-8859-1?Q?+uRx58W70KLYHXwTWzdPdgNATNAP2MeEuXY6EgbQeGOe8t0hyClXtoJ95a?=
 =?iso-8859-1?Q?wc9MFk1wmTydjaBu98FUhPqRt63yIWrxs7cqrFeLNjAZYTnU+JdlwKxX4n?=
 =?iso-8859-1?Q?Xvfz+BJO8p/nGOisxRD19Nao97H+5oISbECSxoTxdRa8YRukD99aLCC3fp?=
 =?iso-8859-1?Q?LikVf/oixz+c/A/c+J1PQOE7Vm+016ArS3MNiqAEfE0Gr33SCwipQQWq/Q?=
 =?iso-8859-1?Q?2kd4TM5RyDIQd0sNz5iaDqoXhsDPv9wV/AEQdc11JW61jvdXSPY/umwvVo?=
 =?iso-8859-1?Q?Lu8uMpz3DBwtMypTdGrzqWkmTVlDV8A+SzjgE+UYKm4Y4oX/fn7EbFbiZl?=
 =?iso-8859-1?Q?r+DqS5aM/u1YxS49F8Sgq5g8XOelk3ZoBNKP1sEgW3zq0ffO/KUsTne+s4?=
 =?iso-8859-1?Q?l5IS9PFCvwxU0WFoMWVcxr3gJzth9n1hHTzl0z07We3JYXcunODUpN7Mox?=
 =?iso-8859-1?Q?aQvs6Aq60jUFyQhtyZa52uzUPGZAEqshtOFxqHzCJ1RW26PmdGO41sT1Ts?=
 =?iso-8859-1?Q?KpYTJKdRP34tr43984mfiBuCRfYW0gsaATYVBm/s+eEdifR3R7XtwqIkaP?=
 =?iso-8859-1?Q?ngbkOXJoBKhOR/YcyBirwV+1GBTjQstkdRdSUed+sXakHkXcEoSoyQWhyQ?=
 =?iso-8859-1?Q?lsljpOoY0LXuksLh6oe4AuoPxU8hpVlTMKLoUcnuQB3smGyi3deVmIFW9G?=
 =?iso-8859-1?Q?VBS8enu7f/KjgB2VnKHjyh4zd5rk5HyBi2mn28+eItbMEmuUClRxgypTFR?=
 =?iso-8859-1?Q?r8G0CESqAJR/ZOI+t6qym26KWhySPO5Vf177kMGdZQUNk6v80s9fKZY/Pl?=
 =?iso-8859-1?Q?zgyiiHa5pHhVqtsBRbAAP9co0VoM7o3/mXL8ACBCKaWU3z9J2tr9UwtRIh?=
 =?iso-8859-1?Q?AeY3a7eq3OFxlrM1v/54q7rp09luCrBeGYtC2yyFjlQHVAdDMhylsPCezy?=
 =?iso-8859-1?Q?uWndWsWec0Srks4aSDjMwnW6E3VqlXbRl/vS7Bh6jifjxb4eQerKDVyLee?=
 =?iso-8859-1?Q?ND5L2W1GNrXTJVgXVHuuPJiyjs7dLyw7EAL45iLZOOi92x3WbqUdTHdWaR?=
 =?iso-8859-1?Q?7i6TSM13PrxFZ1Ffq+AngANk9sloXf7tT1gRDiGP3+ySs+rVXTvV7lj7nW?=
 =?iso-8859-1?Q?bga2hzMN0UsUkkMRlGEgQYtiWEVyth0ANOUxbiSYlxH47SZqQO+yJxWSvF?=
 =?iso-8859-1?Q?tGOgGMgm9KJaKS6FMdLhWdMSCsIoVzavUiY3PFi/ccsAPTGcIWTzdRVw?=
 =?iso-8859-1?Q?=3D=3D?=
Content-Type: text/plain; charset="iso-8859-1"
Content-Transfer-Encoding: quoted-printable
MIME-Version: 1.0
X-OriginatorOrg: forkingsoftware.com
X-MS-Exchange-CrossTenant-AuthAs: Internal
X-MS-Exchange-CrossTenant-AuthSource: SJ0PR18MB4899.namprd18.prod.outlook.com
X-MS-Exchange-CrossTenant-Network-Message-Id: f9dc7077-113f-459b-9fc2-08da56070b9c
X-MS-Exchange-CrossTenant-originalarrivaltime: 24 Jun 2022 17:29:08.5763
 (UTC)
X-MS-Exchange-CrossTenant-fromentityheader: Hosted
X-MS-Exchange-CrossTenant-id: af83f966-c7f0-4e4c-aabd-e6e05bc57733
X-MS-Exchange-CrossTenant-mailboxtype: HOSTED
X-MS-Exchange-CrossTenant-userprincipalname: JrcEfSyScdnPlLJ9yltMMOOKid5fu4LRIY9tcFZy2j2/XKQQ1POra4rfJ/PDDnyDok7zdkOu3xUgoeU2sgRSEyNG+NRx80By/6/8s8LljtU=
X-MS-Exchange-Transport-CrossTenantHeadersStamped: MN2PR18MB3421

`
)

func TestPlainHTMLParsing(t *testing.T) {
	msg, err := smtpd.NewMessage(nil, []byte(plainHTMLEmail), nil, nil)

	if err != nil {
		t.Error("error creating message", err)
		return
	}

	expectTo := []mail.Address{
		{
			Name:    "",
			Address: "recipient1@example.com",
		},
		{
			Name:    "Recipient 2",
			Address: "recipient2@example.com",
		},
	}

	if len(msg.To) < len(expectTo) {
		t.Errorf("Not enough recipients, want: %v, got: %v", len(expectTo), len(msg.To))

	}

	for i, expect := range expectTo {
		if i >= len(msg.To) {
			break
		}
		if msg.To[i].Address != expect.Address || msg.To[i].Name != expect.Name {
			t.Errorf("Wrong recipient %v want: %v, got: %v", i, expect, msg.To[i])
		}
	}

	expectHTML := `<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>🐝
  </body>
</html>`

	if html, err := msg.HTML(); err != nil {
		t.Error(err)
	} else if strings.TrimSpace(string(html)) != expectHTML {
		t.Errorf("Wrong HTML content, want: %v, got: %v", expectHTML, strings.TrimSpace(string(html)))
	}

	if plain, err := msg.Plain(); err == nil {
		t.Error("Expected plaintext version to be missing, got:", plain)
	}
}

func TestAlternativeMessageParsing(t *testing.T) {
	msg, err := smtpd.NewMessage(nil, []byte(alternativeEmail), nil, nil)

	if err != nil {
		t.Error("error creating message", err)
		return
	}

	expectTo := []mail.Address{
		{
			Name:    "",
			Address: "recipient1@example.com",
		},
		{
			Name:    "Recipient 2",
			Address: "recipient2@example.com",
		},
	}

	if len(msg.To) < len(expectTo) {
		t.Errorf("Not enough recipients, want: %v, got: %v", len(expectTo), len(msg.To))

	}

	for i, expect := range expectTo {
		if i >= len(msg.To) {
			break
		}
		if msg.To[i].Address != expect.Address || msg.To[i].Name != expect.Name {
			t.Errorf("Wrong recipient %v want: %v, got: %v", i, expect, msg.To[i])
		}
	}

	expectHTML := `<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>🐝
  </body>
</html>`

	if html, err := msg.HTML(); err != nil {
		t.Error(err)
	} else if strings.TrimSpace(string(html)) != expectHTML {
		t.Errorf("Wrong HTML content, want: %v, got: %v", expectHTML, strings.TrimSpace(string(html)))
	}

	expectPlain := `Sending bees

🐝`

	if plain, err := msg.Plain(); err != nil {
		t.Error(err)
	} else if strings.TrimSpace(string(plain)) != expectPlain {
		t.Errorf("Wrong Plaintext content, want: %v, got: %v", expectPlain, strings.TrimSpace(string(plain)))
	}
}

func TestEmptyBodyMessageParsingDoesNotCrash(t *testing.T) {
	msg, err := smtpd.NewMessage(nil, []byte(emailWithNoBody), nil, nil)
	if err != nil {
		t.Error("error creating message without body", err)
		return
	}

	if len(msg.To) < 1 {
		t.Errorf("Not enough recipients, want: %v, got: %v", 1, len(msg.To))
	}
	if msg.From.Address != "team@forkingsoftware.com" {
		t.Errorf("Expected parsed message to have FROM but got %+v", msg.From)
	}
}

func TestMixedMessageParsing(t *testing.T) {

	msg, err := smtpd.NewMessage(nil, []byte(emailWithAttachment), nil, nil)

	if err != nil {
		t.Error("error creating message", err)
		return
	}

	expectTo := []mail.Address{
		{
			Name:    "",
			Address: "recipient1@example.com",
		},
		{
			Name:    "Recipient 2",
			Address: "recipient2@example.com",
		},
	}

	if len(msg.To) < len(expectTo) {
		t.Errorf("Not enough recipients, want: %v, got: %v", len(expectTo), len(msg.To))

	}

	for i, expect := range expectTo {
		if i >= len(msg.To) {
			break
		}
		if msg.To[i].Address != expect.Address || msg.To[i].Name != expect.Name {
			t.Errorf("Wrong recipient %v want: %v, got: %v", i, expect, msg.To[i])
		}
	}

	expectHTML := `<!DOCTYPE html>
<html>
  <body>
    Sending bees<br><br>🐝
  </body>
</html>`

	if html, err := msg.HTML(); err != nil {
		t.Error(err)
	} else if strings.TrimSpace(string(html)) != expectHTML {
		t.Errorf("Wrong HTML content, want: %v, got: %v", expectHTML, strings.TrimSpace(string(html)))
	}

	expectPlain := `Sending bees

🐝`

	if plain, err := msg.Plain(); err != nil {
		t.Error(err)
	} else if strings.TrimSpace(string(plain)) != expectPlain {
		t.Errorf("Wrong Plaintext content, want: %v, got: %v", expectPlain, strings.TrimSpace(string(plain)))
	}

	// TODO: check rest of parse proceeded as expected
	var attachments []*smtpd.Part
	if attachments, err = msg.Attachments(); err != nil {
		t.Error("couldn't load attachments", err)
	}

	if len(attachments) != 1 {
		t.Errorf("want one attachment, got: %v", len(attachments))
	}

	mimeType, _, err := mime.ParseMediaType(attachments[0].Header.Get("Content-Type"))
	if err != nil {
		t.Error("Error parsing attachment MIME header:", err)
	}

	if mimeType != "text/calendar" {
		t.Errorf("Expected text/calendar attachment, got: %v", mimeType)
	}

	expectVCal := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//mailproto//MailProto
CALSCALE:GREGORIAN
BEGIN:VEVENT
DTSTAMP:20170116T154000
UID:mycoolevent@mailproto

DTSTART;TZID="America/New_York":20170118T110000
DTEND;TZID="America/New_York":20170118T120000
SUMMARY:Send an email
LOCATION:Test
END:VEVENT
END:VCALENDAR`

	if string(attachments[0].Body) != expectVCal {
		t.Errorf("Wrong attachment, wanted: %v got: %v", expectVCal, string(attachments[0].Body))
	}

}

func TestInvalidEmailBodyStillPassesToHandler(t *testing.T) {

	msg, err := smtpd.NewMessage(nil, []byte(emailWithInvalidBody), nil, nil)

	if err != nil {
		t.Error("error creating message", err)
		return
	}

	expectTo := []mail.Address{
		{
			Name:    "",
			Address: "recipient1@example.com",
		},
		{
			Name:    "Recipient 2",
			Address: "recipient2@example.com",
		},
	}

	if len(msg.To) < len(expectTo) {
		t.Errorf("Not enough recipients, want: %v, got: %v", len(expectTo), len(msg.To))

	}

	for i, expect := range expectTo {
		if i >= len(msg.To) {
			break
		}
		if msg.To[i].Address != expect.Address || msg.To[i].Name != expect.Name {
			t.Errorf("Wrong recipient %v want: %v, got: %v", i, expect, msg.To[i])
		}
	}

	_, err = msg.Parts()
	if err == nil {
		t.Error("Expected parts parsing to fail due to invalid body")
	}
}

func TestUTFEncodingInFromName(t *testing.T) {
	msg, err := smtpd.NewMessage(nil, []byte(utf8EncodedFromName), nil, nil)

	if err != nil {
		t.Error("error creating message", err)
		return
	}

	expectFrom := []mail.Address{
		{
			Name:    "Sender \\u0014\\",
			Address: "sender@example.com",
		},
	}

	if msg.From.Name != expectFrom[0].Name {
		t.Errorf("Wrong from name want: %v, got %v", expectFrom[0].Name, msg.From.Name)
	}
}
