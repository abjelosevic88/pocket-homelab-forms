package server

// siteWords are the few words a public form page says itself, per language
// (the questions are the owner's own words).
var siteWords = map[string]map[string]string{
	"en": {
		"lang": "English", "required": "Required", "optional": "optional", "submit": "Send", "other": "Other", "other_ph": "Your answer",
		"choose": "Choose…", "yes": "Yes", "no": "No", "thanks_title": "Thank you!", "thanks": "Your answers were sent.",
		"again": "Send another answer", "closed_title": "This form is closed", "closed": "It does not take answers any more.",
		"closed_date": "It took answers until %s.", "full": "It has all the answers it needs.", "notfound": "There is no form at this address.",
		"preview": "Preview — only you see this. Open the form to take answers.", "preview_sent": "This was a preview, so nothing was saved.",
		"e_required": "Please answer this question.", "e_email": "This does not look like an email address.", "e_number": "Please write a number.",
		"e_url": "Please write a web address.", "e_date": "Please pick a day.", "e_time": "Please pick a time.", "e_choice": "Please pick one of the choices.",
		"e_phone": "This does not look like a phone number.", "e_long": "This answer is too long.", "fix": "Please look at the questions marked below.",
		"already": "This email address has already answered. Thank you!", "wait": "Many answers came from here just now. Please try again in a few minutes.",
		"stamp": "The page was open for a long time, so please send it once more.", "made": "Made with Pocket Forms", "rating": "%d of 5",
		"sent_by": "Answers to “%s”", "copy_intro": "Here is a copy of what you sent.",
	},
	"bs": {
		"lang": "Bosanski", "required": "Obavezno", "optional": "nije obavezno", "submit": "Pošalji", "other": "Drugo", "other_ph": "Vaš odgovor",
		"choose": "Odaberite…", "yes": "Da", "no": "Ne", "thanks_title": "Hvala!", "thanks": "Vaši odgovori su poslani.",
		"again": "Pošalji još jedan odgovor", "closed_title": "Ovaj obrazac je zatvoren", "closed": "Više ne prima odgovore.",
		"closed_date": "Primao je odgovore do %s.", "full": "Ima sve odgovore koji su mu potrebni.", "notfound": "Na ovoj adresi nema obrasca.",
		"preview": "Pregled — vidite ga samo vi. Otvorite obrazac da bi primao odgovore.", "preview_sent": "Ovo je bio pregled, ništa nije sačuvano.",
		"e_required": "Molimo odgovorite na ovo pitanje.", "e_email": "Ovo ne izgleda kao e-mail adresa.", "e_number": "Molimo upišite broj.",
		"e_url": "Molimo upišite web adresu.", "e_date": "Molimo odaberite dan.", "e_time": "Molimo odaberite vrijeme.", "e_choice": "Molimo odaberite jedan od ponuđenih odgovora.",
		"e_phone": "Ovo ne izgleda kao broj telefona.", "e_long": "Odgovor je predug.", "fix": "Molimo pogledajte označena pitanja.",
		"already": "S ove e-mail adrese je već odgovoreno. Hvala!", "wait": "Odavde je upravo stiglo mnogo odgovora. Pokušajte ponovo za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Napravljeno u Pocket Forms", "rating": "%d od 5",
		"sent_by": "Odgovori na „%s”", "copy_intro": "Ovo je kopija onoga što ste poslali.",
	},
	"hr": {
		"lang": "Hrvatski", "required": "Obavezno", "optional": "nije obavezno", "submit": "Pošalji", "other": "Drugo", "other_ph": "Vaš odgovor",
		"choose": "Odaberite…", "yes": "Da", "no": "Ne", "thanks_title": "Hvala!", "thanks": "Vaši su odgovori poslani.",
		"again": "Pošalji još jedan odgovor", "closed_title": "Ovaj je obrazac zatvoren", "closed": "Više ne prima odgovore.",
		"closed_date": "Primao je odgovore do %s.", "full": "Ima sve odgovore koji su mu potrebni.", "notfound": "Na ovoj adresi nema obrasca.",
		"preview": "Pregled — vidite ga samo vi. Otvorite obrazac da bi primao odgovore.", "preview_sent": "Ovo je bio pregled, ništa nije spremljeno.",
		"e_required": "Molimo odgovorite na ovo pitanje.", "e_email": "Ovo ne izgleda kao adresa e-pošte.", "e_number": "Molimo upišite broj.",
		"e_url": "Molimo upišite web-adresu.", "e_date": "Molimo odaberite dan.", "e_time": "Molimo odaberite vrijeme.", "e_choice": "Molimo odaberite jedan od ponuđenih odgovora.",
		"e_phone": "Ovo ne izgleda kao broj telefona.", "e_long": "Odgovor je predug.", "fix": "Molimo pogledajte označena pitanja.",
		"already": "S ove adrese e-pošte već je odgovoreno. Hvala!", "wait": "Odavde je upravo stiglo mnogo odgovora. Pokušajte ponovno za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Izrađeno u Pocket Forms", "rating": "%d od 5",
		"sent_by": "Odgovori na „%s”", "copy_intro": "Ovo je kopija onoga što ste poslali.",
	},
	"sr": {
		"lang": "Srpski", "required": "Obavezno", "optional": "nije obavezno", "submit": "Pošalji", "other": "Drugo", "other_ph": "Vaš odgovor",
		"choose": "Izaberite…", "yes": "Da", "no": "Ne", "thanks_title": "Hvala!", "thanks": "Vaši odgovori su poslati.",
		"again": "Pošalji još jedan odgovor", "closed_title": "Ovaj obrazac je zatvoren", "closed": "Više ne prima odgovore.",
		"closed_date": "Primao je odgovore do %s.", "full": "Ima sve odgovore koji su mu potrebni.", "notfound": "Na ovoj adresi nema obrasca.",
		"preview": "Pregled — vidite ga samo vi. Otvorite obrazac da bi primao odgovore.", "preview_sent": "Ovo je bio pregled, ništa nije sačuvano.",
		"e_required": "Molimo odgovorite na ovo pitanje.", "e_email": "Ovo ne izgleda kao imejl adresa.", "e_number": "Molimo upišite broj.",
		"e_url": "Molimo upišite veb adresu.", "e_date": "Molimo izaberite dan.", "e_time": "Molimo izaberite vreme.", "e_choice": "Molimo izaberite jedan od ponuđenih odgovora.",
		"e_phone": "Ovo ne izgleda kao broj telefona.", "e_long": "Odgovor je predugačak.", "fix": "Molimo pogledajte označena pitanja.",
		"already": "Sa ove imejl adrese je već odgovoreno. Hvala!", "wait": "Odavde je upravo stiglo mnogo odgovora. Pokušajte ponovo za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Napravljeno u Pocket Forms", "rating": "%d od 5",
		"sent_by": "Odgovori na „%s”", "copy_intro": "Ovo je kopija onoga što ste poslali.",
	},
	"de": {
		"lang": "Deutsch", "required": "Pflichtfeld", "optional": "freiwillig", "submit": "Senden", "other": "Sonstiges", "other_ph": "Ihre Antwort",
		"choose": "Bitte wählen…", "yes": "Ja", "no": "Nein", "thanks_title": "Vielen Dank!", "thanks": "Ihre Antworten wurden gesendet.",
		"again": "Noch eine Antwort senden", "closed_title": "Dieses Formular ist geschlossen", "closed": "Es nimmt keine Antworten mehr an.",
		"closed_date": "Es nahm Antworten bis zum %s an.", "full": "Es hat alle Antworten, die es braucht.", "notfound": "Unter dieser Adresse gibt es kein Formular.",
		"preview": "Vorschau — nur Sie sehen das. Öffnen Sie das Formular, um Antworten anzunehmen.", "preview_sent": "Das war eine Vorschau, es wurde nichts gespeichert.",
		"e_required": "Bitte beantworten Sie diese Frage.", "e_email": "Das sieht nicht wie eine E-Mail-Adresse aus.", "e_number": "Bitte geben Sie eine Zahl ein.",
		"e_url": "Bitte geben Sie eine Webadresse ein.", "e_date": "Bitte wählen Sie einen Tag.", "e_time": "Bitte wählen Sie eine Uhrzeit.", "e_choice": "Bitte wählen Sie eine der Antworten.",
		"e_phone": "Das sieht nicht wie eine Telefonnummer aus.", "e_long": "Die Antwort ist zu lang.", "fix": "Bitte sehen Sie sich die markierten Fragen an.",
		"already": "Von dieser E-Mail-Adresse kam schon eine Antwort. Danke!", "wait": "Von hier kamen gerade viele Antworten. Bitte versuchen Sie es in ein paar Minuten noch einmal.",
		"stamp": "Die Seite war lange offen, bitte senden Sie sie noch einmal.", "made": "Erstellt mit Pocket Forms", "rating": "%d von 5",
		"sent_by": "Antworten auf „%s“", "copy_intro": "Hier ist eine Kopie Ihrer Antworten.",
	},
}

// word is one word in a language (English when the language lacks it).
func word(lang, key string) string {
	if w, ok := siteWords[lang][key]; ok {
		return w
	}
	return siteWords["en"][key]
}
