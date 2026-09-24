package scan_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
)

// evaluation is one case of the set the default configuration's weights and thresholds were set
// against. A body case expects the one-time code and login link flags. A subject case expects
// whether anything in it is masked.
type evaluation struct {
	name, text     string
	subject        bool
	mfa, loginLink bool
}

// evaluationSet holds one-time codes and login links written the way the default templates of common
// authentication libraries and services write them, and mail that carries numbers, alphanumeric
// references and long links that are not secrets. The values are synthetic.
func evaluationSet() []evaluation {
	body := func(name, text string, mfa, loginLink bool) evaluation {
		return evaluation{name: name, text: text, mfa: mfa, loginLink: loginLink}
	}
	subject := func(name, text string, masked bool) evaluation {
		return evaluation{name: "in a subject, " + name, text: text, subject: true, mfa: masked}
	}
	return []evaluation{
		// One-time codes in bodies.
		body("a code on its own line after the instruction", "Please recover access to your account by entering the following code:\n\n830719\n", true, false),
		body("a code as a heading", "## 482913\n\nThis code expires in 10 minutes.", true, false),
		body("a code at the end of a sentence", "The verification code to your new account is 582033", true, false),
		body("a code alone in bold", "Your email verification code is\n\n**572910**\n", true, false),
		body("a code after a colon", "Enter the code: 902341", true, false),
		body("a code under a label", "Please use the following single-use code for your account.\n\nSecurity code: 519034\n", true, false),
		body("a code in groups of three", "Your code is 519 034.", true, false),
		body("a code in groups joined by a hyphen", "Your code: 519-034", true, false),
		body("a code before its trigger", "747723 is your Example authentication code.", true, false),
		body("a code as a table cell", "| Your code |\n| --- |\n| 318204 |", true, false),
		body("a code with a letter prefix", "G-845213 is your Example verification code.", true, false),
		body("an eight-digit code", "One-Time Code:73920418", true, false),
		body("a code after a one-time password", "ABC-482913 is your one-time password.", true, false),
		body("an alphanumeric code alone in bold", "Use this code to sign in.\n\n**7GX4Q2**\n", true, false),
		body("an alphanumeric code after a trigger", "Your confirmation code is K7P2QX.", true, false),
		body("an alphanumeric code on its own line", "Sign in to Example\n\nA9F3KD\n\nIt expires in 15 minutes.", true, false),
		body("an alphanumeric login code", "Your login code for Example is 6MK4TZ", true, false),
		// Login links in bodies.
		body("a password reset parameter", "[Change my password](https://app.example/users/password/edit?reset_password_token=8hSzj0vcS8biN8Dh2mHL)", false, true), // gitleaks:allow, a synthetic value
		body("a confirmation parameter", "[Confirm my account](https://app.example/users/confirmation?confirmation_token=pyC0IoUlUpvC5BNyyil2)", false, true),      // gitleaks:allow, a synthetic value
		body("a token under a passwords path", "https://app.example/passwords/eyJfcmFpbHMiOnsiZGF0YSI6MTJ9fQ--4f1c7a9e2b/edit", false, true),
		body("a reset path with a hex token", "https://app.example/reset/MTI/c3k9zj-e258f31aa2c9b4451755d43c97d8be1d/", false, true),
		body("a reset path with an email parameter", "https://app.example/reset-password/1f0637957d737f8ea1f939045844ebe83cc7d35d80c5d6dd3bfcaff4f3db2d29?email=someone%40mail.example", false, true),
		body("a signed verification route", "https://app.example/email/verify/42/291c7b0f6dba6d8cc60f42f6f7476d7b2bc70e2a?expires=1727190000&signature=1f0637957d737f8ea1f939045844ebe83cc7d35d80c5d6dd3bfcaff4f3db2d29", false, true),
		body("a callback with a token", "https://app.example/api/auth/callback/email?callbackUrl=https%3A%2F%2Fapp.example&token=1f0637957d737f8ea1f939045844ebe83cc7d35d80c5d6dd3bfcaff4f3db2d29&email=someone%40mail.example", false, true),
		body("a magic link of letters", "https://app.example/magic-link/verify?token=vqqmgaqxwriDaSkbfyzCLmallXNeXtOO&callbackURL=/", false, true),
		body("a hosted verify endpoint", "https://ref.auth.example/auth/v1/verify?token=d485e2e964bbf361f1a37496639619f6cf701e8e69ec8a864655e42b&type=email", false, true),
		body("an out-of-band code", "https://app.example/__/auth/action?mode=resetPassword&oobCode=vYBuFicdtyjvVnzEf1SMEOe2hAdFmZvvfsOOzypHzHxf20bxFwFlbQ&lang=en", false, true),
		body("an action token", "https://id.example/realms/main/login-actions/action-token?key=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.4_M73e31OEFBef-td46CnJIbdXFDTnORdJ-UzHQD2YnEHOMySO8IwVbjLILpin2Ytikzyrr2YndrA3Eq.L0AVz2uxBpLBCTeWPoNpepGIgJNXbvPPgwLUe0d56TU&client_id=app", false, true), // gitleaks:allow, a synthetic value
		body("a recovery flow", "https://app.example/self-service/recovery?flow=6c1f3a52-8a0e-4d0b-9f67-2b1e5c7d9a41&token=VWmYIFUm09o919N58gOF8GyjXcjZI8tz", false, true),                                                                                                                    // gitleaks:allow, a synthetic value
		body("a password reset key", "https://blog.example/wp-login.php?login=someone&key=pyC0IoUlUpvC5BNyyil2&action=rp", false, true),
		body("an email login path", "https://forum.example/session/email-login/e258f31aa2c9b4451755d43c97d8be1d", false, true),
		body("a confirmation code parameter", "https://app.example/Identity/Account/ConfirmEmail?userId=42&code=KMXcVfC-kBgNr1oxvKrJz3_rnSkxEOysPDiYIHTmre4J", false, true),
		body("a magic link token", "https://app.example/authenticate?stytch_token_type=magic_links&token=KMXcVfC-kBgNr1oxvKrJz3_rnSkxEOysPDiYIHTmre4J", false, true), // gitleaks:allow, a synthetic value
		body("a verification ticket", "https://tenant.example/u/email-verification?ticket=vYBuFicdtyjvVnzEf1SMEOe2hAdFmZvvfsOOzypHzHxf20bxFwFlbQ", false, true),
		// Bodies holding nothing secret.
		body("an order number", "Your order #48213092 has shipped.", false, false),
		body("an alphanumeric order number", "Order A1B2C3D4 shipped today.", false, false),
		body("a phone number", "Questions? Call us at 555 012 3456.", false, false),
		body("a date", "The sale ends 2026-09-30.", false, false),
		body("a flight number", "Flight UA1234 departs at 10:45.", false, false),
		body("a product reference", "Item SKU K2X9P4 is back in stock.", false, false),
		body("a price table", "| Item | Price |\n| --- | --- |\n| Lamp | 42.00 |\n| Total | 42.00 |", false, false),
		body("a marketing link", "https://shop.example/spring-sale-2026-newsletter?utm_campaign=spring_sale_2026_newsletter&utm_source=email", false, false),
		body("an unsubscribe link", "https://news.example/unsubscribe?id=8hSzj0vcS8biN8Dh2mHL", false, false),
		body("a wrapped tracking link", "https://u123.ct.mail.example/ls/click?upn=RLzTX-xfZBuZ95FncQgJ8XEqzhjVQ8b039RlPC79fC5eTKb5xenK60BnbxlB1_Tl1rZ_MRwC-crUb3xaOv5kBQouzB1qUvXiFKtn3uevhS5G1mXcEmvpB7hY", false, false),
		// A slug holding a link word is flagged, because a long slug is as dense as a token.
		body("an article slug holding a link word", "https://news.example/2026/09/how-to-reset-your-sleep-schedule-before-winter", false, true),
		body("a year in prose", "Thanks for being with us since 2019.", false, false),
		// Subjects.
		subject("a code before its trigger", "482913 is your Example login code", true),
		subject("a code after an instruction", "Use code 830719 to log in", true),
		subject("a code after a trigger", "Your verification code is 582033", true),
		subject("an alphanumeric code after a trigger", "Example sign-in code: K7P2QX", true),
		subject("a code with a letter prefix", "G-845213 is your verification code", true),
		subject("a code request without a code", "Your single-use code", false),
		subject("an order number", "Your order #48213092 has shipped", false),
		subject("a reservation number", "Reservation 48213 is confirmed", false),
		subject("an alphanumeric order number", "Your order A1B2C3 has shipped", false),
		subject("a flight number", "Flight UA1234 check-in is open", false),
		subject("an invoice number", "Invoice 2024-0917 is ready", false),
	}
}

// The default configuration flags every one-time code and login link in the evaluation set, and of
// everything else in it only the slug its link rule cannot tell from a token.
func TestEvaluationSet(t *testing.T) {
	s := scanner(t)
	for _, e := range evaluationSet() {
		t.Run(e.name, func(t *testing.T) {
			mfa, loginLink := evaluate(s, e)
			if mfa != e.mfa || loginLink != e.loginLink {
				t.Errorf("%q gave a one-time code %v and a login link %v, want %v and %v", e.text, mfa, loginLink, e.mfa, e.loginLink)
			}
		})
	}
}

// evaluate returns what s finds in e, the verdict's flags on a body, and whether anything is masked
// in a subject.
func evaluate(s scan.Scanner, e evaluation) (mfa, loginLink bool) {
	if e.subject {
		spans, _ := s.SubjectSpans(e.text)
		return len(spans) > 0, false
	}
	f := s.Scan(e.text).Flags()
	return f.MFACode(), f.LoginLink()
}
