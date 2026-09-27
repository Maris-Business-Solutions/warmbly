package replyclassify

import (
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

// Every tagging language carries opt-out phrases, and every language with
// phrases has quote markers to cut its campaigns' own opt-out line with.
func TestOptOutCoversEveryTaggingLanguage(t *testing.T) {
	for code := range models.MailLanguageNames {
		if code == "en" || code == "de" {
			continue
		}
		if len(optOutPhrases[code]) == 0 {
			t.Errorf("%s: no opt-out phrases", code)
		}
	}
	for _, code := range optOutLanguages {
		if len(languageRules[code].quote) == 0 {
			t.Errorf("%s: opt-out phrases but no quote markers", code)
		}
	}
}

// Each phrase opts out as the reply writes it, with the suffix or prefix its
// "*" leaves room for, in upper case and wrapped onto the next line.
func TestEveryOptOutPhraseMatchesItsReply(t *testing.T) {
	for code, phrases := range optOutPhrases {
		for _, p := range phrases {
			word := strings.Trim(p, "*")
			if strings.HasSuffix(p, "*") {
				word += "in"
			}
			if strings.HasPrefix(p, "*") {
				word = "و" + word
			}
			for _, body := range []string{
				"Hello,\n" + word + "\nThanks",
				strings.ToUpper(word) + ".",
				strings.Replace(word, " ", "\n", 1) + ".",
			} {
				if !IsOptOut("Re: Hello", body) {
					t.Errorf("%s: %q not an opt-out in %q", code, p, body)
				}
			}
		}
	}
}

// Replies in each language, as people write them.
func TestIsOptOutInEveryLanguage(t *testing.T) {
	optOut := map[string][]string{
		"ar":  {"من فضلكم احذفوني من القائمة", "أرجو إلغاء الاشتراك", "لا ترسلوا لي أي رسائل أخرى"},
		"bg":  {"Моля, отпишете ме от списъка.", "Не ми пишете повече."},
		"bn":  {"দয়া করে আমাকে তালিকা থেকে বাদ দিন।"},
		"ca":  {"Si us plau, doneu-me de baixa.", "No m'envieu més correus."},
		"cs":  {"Prosím odhlaste mě z odběru.", "Neposílejte mi už žádné e-maily."},
		"da":  {"Fjern mig venligst fra listen.", "Jeg ønsker ikke at modtage flere mails."},
		"el":  {"Παρακαλώ διαγράψτε με από τη λίστα.", "ΣΤΑΜΑΤΗΣΤΕ ΝΑ ΜΟΥ ΣΤΕΛΝΕΤΕ"},
		"es":  {"Por favor, denme de baja.", "No me envíen más correos.", "Quitenme de su lista"},
		"et":  {"Palun eemaldage mind nimekirjast.", "Ärge saatke mulle enam kirju."},
		"fa":  {"لطفا مرا از لیست حذف کنید.", "دیگر ایمیل نفرستید"},
		"fi":  {"Poistakaa minut listaltanne.", "Älkää lähettäkö minulle enää viestejä."},
		"fil": {"Pakiusap, alisin ako sa listahan.", "Huwag na akong padalhan ng email."},
		"fr":  {"Merci de me désinscrire.", "Ne m’envoyez plus de messages.", "Desabonnez-moi svp"},
		"he":  {"בבקשה הסירו אותי מהרשימה", "ותפסיקו לשלוח לי מיילים"},
		"hi":  {"कृपया मुझे सूची से हटाएं।", "मुझे ईमेल मत भेजो"},
		"hr":  {"Molim vas, odjavite me.", "Nemojte mi slati više poruka."},
		"hu":  {"Kérem, iratkoztasson le a listáról.", "Leiratkozom.", "Ne küldjenek több levelet."},
		"id":  {"Tolong hapus saya dari daftar.", "Jangan kirim email lagi."},
		"it":  {"Per favore cancellatemi dalla lista.", "Non inviatemi più email."},
		"ja":  {"配信停止をお願いします。", "今後メールを送らないでください。"},
		"ko":  {"수신거부합니다.", "더 이상 연락하지 마세요."},
		"lt":  {"Prašau išbraukite mane iš sąrašo.", "Nebesiųskite man laiškų."},
		"lv":  {"Lūdzu, izņemiet mani no saraksta.", "Nesūtiet man vairs e-pastus."},
		"ms":  {"Sila keluarkan saya dari senarai.", "Jangan hantar e-mel lagi."},
		"nb":  {"Vennligst fjern meg fra listen.", "Ikke kontakt meg igjen."},
		"nl":  {"Graag afmelden.", "Verwijder mij uit uw bestand.", "Stuur me geen mails meer."},
		"pl":  {"Proszę mnie wypisać z listy.", "Nie wysyłajcie mi więcej wiadomości."},
		"pt":  {"Por favor, me remova da lista.", "Não me enviem mais e-mails.", "Nao quero receber mais"},
		"ro":  {"Vă rog să mă scoateți-mă din listă.", "Nu-mi mai trimiteți emailuri.", "Stergeti-ma"},
		"ru":  {"Пожалуйста, отпишите меня.", "Больше не пишите мне.", "Удалите мой адрес из базы."},
		"sk":  {"Prosím, odhláste ma.", "Neposielajte mi už e-maily."},
		"sl":  {"Prosim, odjavite me.", "Ne pošiljajte mi več sporočil."},
		"sr":  {"Молим вас, одјавите ме.", "Ne šaljite mi više poruke."},
		"sv":  {"Ta bort mig från listan.", "Sluta skicka mejl till mig."},
		"sw":  {"Tafadhali niondoe kwenye orodha.", "Acheni kunitumia barua pepe."},
		"ta":  {"தயவுசெய்து என்னை பட்டியலிலிருந்து நீக்கவும்."},
		"th":  {"กรุณาลบฉันออกจากรายชื่อ", "ไม่ต้องส่งมาอีกนะครับ"},
		"tr":  {"Lütfen beni listeden çıkarın.", "Abonelikten çıkmak istiyorum.", "Bana mail atmayın."},
		"uk":  {"Будь ласка, відпишіть мене.", "Більше не надсилайте листів."},
		"ur":  {"براہ کرم مجھے فہرست سے نکال دیں۔"},
		"vi":  {"Vui lòng xóa tôi khỏi danh sách.", "Đừng gửi email cho tôi nữa.", "Huy dang ky"},
		"zh":  {"请把我从邮件列表中删除。", "請不要再寄信給我。", "取消订阅"},
	}
	for code, replies := range optOut {
		for _, r := range replies {
			if !IsOptOut("Re: Hello", r) {
				t.Errorf("%s: %q not read as an opt-out", code, r)
			}
		}
	}
	// A word that also cancels a meeting, a leave, or means "don't worry" is
	// not a request to stop.
	for _, r := range []string{
		"Estoy de baja hasta el lunes, le escribo luego.",
		"Ik moet me afmelden voor de webinar van morgen.",
		"Envoyez-moi plus de mails sur ce sujet, merci.",
		"Не беспокойтесь, всё в порядке.",
		"資料は送らないでください、リンクで十分です。",
		"Posso cancellare la riunione di domani?",
		"Kan du avmelde møtet på fredag?",
		"Moram otkazati sastanak, javim se.",
		"Мы удалили старую версию, пришлите новую.",
		"Interesante, hablemos la próxima semana.",
		"Vielen Dank, das klingt gut.",
		"Не присылайте счёт, мы уже оплатили.",
		"Не пишите мне сюда, лучше на рабочую почту.",
		"Stuur me geen factuur, die heb ik al.",
		"電話で連絡しないでください、メールでお願いします。",
		"Proszę nie wysyłać faktury papierowej.",
		"Neposílejte mi fakturu poštou.",
		"Älä lähetä minulle laskua paperilla.",
		"אל תשלחו לי חשבונית, כבר שילמנו.",
	} {
		if IsOptOut("Re: Hello", r) {
			t.Errorf("%q read as an opt-out", r)
		}
	}
}

// A campaign's own opt-out line, quoted under the reply in its language's
// attribution, is the sender's text and never opts the replier out.
func TestQuotedOptOutLineInEveryLanguage(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range attributionCases {
		phrases := optOutPhrases[c.lang]
		if len(phrases) == 0 || seen[c.lang] {
			continue
		}
		seen[c.lang] = true
		line := "Reply «" + strings.Trim(phrases[0], "*") + "» to stop."
		body := "Thanks, sounds good.\n\n" + c.line + "\nHello Anna, a short note.\n" + line
		if IsOptOut("Re: Hello", body) {
			t.Errorf("%s: the quoted opt-out line opted out: %q", c.lang, body)
		}
		if !IsOptOut("Re: Hello", strings.Trim(phrases[0], "*")+"\n\n"+c.line+"\nHello Anna.") {
			t.Errorf("%s: the reply's own opt-out was cut", c.lang)
		}
	}
	for code := range optOutPhrases {
		if !seen[code] {
			t.Errorf("%s: no attribution case to quote under", code)
		}
	}
}
