package replyclassify

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// optOutPhrases are opt-out requests in the tagging languages besides English
// and German, which unsubscribeKeywords carries. Each is written as a person
// types it and folded like the reply before matching. Every one names the
// writer or the mail, so a word that also cancels a meeting or an order never
// counts alone. A "*" at either end leaves that end open for a prefix or a
// suffix the language attaches to the word ("abonelikten çık*" reads
// "çıkarın").
var optOutPhrases = map[string][]string{
	"ar": {
		"إلغاء الاشتراك", "الغي اشتراكي", "ألغوا اشتراكي", "*احذفوني", "*احذفني", "احذف بريدي", "احذفوا بريدي",
		"*أزيلوني", "أزيلوا بريدي", "*توقفوا عن مراسلتي", "*توقف عن مراسلتي", "*توقفوا عن إرسال",
		"لا ترسلوا لي المزيد", "لا ترسل لي المزيد", "لا ترسلوا لي أي رسائل", "*لا تراسلوني", "لا تتواصلوا معي", "لا تتواصل معي", "احذفوا بياناتي",
	},
	"bg": {
		"отпишете ме", "отпишете ни", "искам да се отпиша", "премахнете ме", "изтрийте ме", "изтрийте данните ми",
		"не ми пишете повече", "не ми изпращайте повече", "спрете да ми изпращате", "спрете да ми пишете", "не се свързвайте с мен",
	},
	"bn": {
		"আনসাবস্ক্রাইব", "সাবস্ক্রিপশন বাতিল*", "আমাকে তালিকা থেকে বাদ*", "আমাকে বাদ দিন", "আমাকে সরিয়ে দিন",
		"ইমেইল পাঠাবেন না", "ইমেল পাঠাবেন না", "আর পাঠাবেন না", "আমার সাথে যোগাযোগ করবেন না", "আমার তথ্য মুছে*",
	},
	"ca": {
		"donar-me de baixa", "doneu-me de baixa", "doni'm de baixa", "vull donar-me de baixa", "baixa de la llista",
		"esborreu-me", "elimineu-me", "traieu-me de", "esborreu les meves dades", "no m'envieu més", "deixeu d'enviar-me",
		"no em contacteu més", "no vull rebre més",
	},
	"cs": {
		"odhlaste mě", "odhlašte mě", "odhlásit z odběru", "odhlásit mě", "zrušit odběr", "zrušte odběr", "odstraňte mě",
		"odstraňte mou adresu", "smažte mě", "smažte mé údaje", "vymažte mé údaje", "neposílejte mi už", "neposílejte mi další", "přestaňte mi posílat",
		"nekontaktujte mě", "nechci dostávat",
	},
	"da": {
		"afmeld mig", "frameld mig", "fjern mig", "slet mig", "slet mine oplysninger", "slet mine data",
		"stop med at sende", "stop med at skrive", "send ikke flere", "ikke flere mails", "ikke flere e-mails",
		"kontakt mig ikke", "kontakt os ikke", "ønsker ikke at modtage",
	},
	"el": {
		"διαγράψτε με", "διαγραφή από τη λίστα", "απεγγραφή", "απεγγράψτε με", "αφαιρέστε με", "σταματήστε να μου στέλνετε",
		"μη μου ξαναστείλετε", "μην μου ξαναστείλετε", "μην επικοινωνείτε μαζί μου", "διαγράψτε τα στοιχεία μου",
	},
	"es": {
		"darme de baja", "dame de baja", "denme de baja", "dadme de baja", "darnos de baja", "de baja de su lista",
		"de baja de la lista", "cancelar la suscripción", "cancelar mi suscripción", "desuscribirme", "desinscribirme",
		"elimínenme", "elimíname", "eliminarme de", "quítenme de", "quítame de", "sáquenme de", "sácame de",
		"borren mis datos", "eliminen mis datos", "no me envíen más", "no me envíes más", "dejen de enviarme", "deja de enviarme",
		"dejen de escribirme", "no me contacten más", "no vuelvan a contactarme", "no quiero recibir más", "no deseo recibir más",
	},
	"et": {
		"loobun tellimusest", "tühistage tellimus", "eemaldage mind", "eemalda mind", "kustutage mind", "kustutage minu andmed",
		"ärge saatke mulle enam", "ärge saatke mulle rohkem", "ära saada mulle enam", "lõpetage kirjade saatmine", "ärge võtke minuga ühendust", "ära võta minuga ühendust",
	},
	"fa": {
		"لغو اشتراک", "لغو عضویت", "مرا از لیست حذف", "من را از لیست حذف", "منو از لیست حذف", "ایمیل من را حذف",
		"دیگر ایمیل نفرستید", "ایمیل نفرستید", "دیگر پیام نفرستید", "با من تماس نگیرید", "اطلاعات من را حذف",
	},
	"fi": {
		"peru tilaus", "peruuta tilaukseni", "poistakaa minut", "poista minut", "poistakaa sähköpostiosoitteeni", "poistakaa tietoni",
		"älkää lähettäkö minulle enää", "älä lähetä minulle enää", "lopettakaa viestien lähettäminen", "älkää ottako minuun yhteyttä",
		"älä ota minuun yhteyttä", "en halua enää viestejä", "en halua enää sähköpost*",
	},
	"fil": {
		"alisin ako sa", "tanggalin ako sa", "huwag na akong padalhan", "huwag nyo na akong padalhan", "huwag niyo na akong padalhan",
		"huwag na akong i-email", "huwag na akong kontakin", "ayoko nang makatanggap", "burahin ang aking data",
	},
	"fr": {
		"désabonner", "désabonnez-moi", "me désabonner", "désinscrire", "désinscrivez-moi", "me désinscrire", "désinscription",
		"retirez-moi de", "retirez mon adresse", "supprimez-moi de", "supprimez mon adresse", "supprimez mes données",
		"ne m'envoyez plus", "arrêtez de m'envoyer", "cessez de m'envoyer", "ne me contactez plus", "ne plus me contacter",
		"ne souhaite plus recevoir", "ne veux plus recevoir", "plus aucun mail", "plus aucun e-mail", "plus aucun email",
	},
	"he": {
		"*הסירו אותי", "*הסר אותי", "*הסירי אותי", "להסיר אותי מהרשימה", "*בטלו את המנוי", "ביטול מנוי", "*אל תשלחו לי יותר",
		"*אל תשלח לי יותר", "*אל תשלחו לי עוד", "*תפסיקו לשלוח", "*אל תפנו אליי", "*אל תיצרו איתי קשר", "*מחקו את הפרטים שלי",
	},
	"hi": {
		"अनसब्सक्राइब", "सदस्यता रद्द*", "मुझे सूची से हटा*", "मुझे हटा दें", "मुझे हटाएं", "मुझे हटाएँ",
		"ईमेल न भेजें", "ईमेल मत भेजो", "ईमेल मत भेजिए", "ईमेल भेजना बंद*", "मुझसे संपर्क न करें", "मेरा डेटा हटा*",
	},
	"hr": {
		"odjavite me", "odjava s liste", "maknite me s", "uklonite me", "izbrišite me", "izbrišite moje podatke",
		"obrišite moje podatke", "ne šaljite mi više", "prestanite mi slati", "nemojte mi više slati", "nemojte mi slati više", "ne kontaktirajte me",
		"nemojte me kontaktirati", "ne pišite mi više",
	},
	"hu": {
		"leiratkoz*", "iratkoztasson le", "iratkoztassanak le", "töröljenek a listá*", "töröljön a listá*",
		"töröljék az adataim*", "töröljenek az adatbázis*", "ne küldjenek több", "ne küldjenek nekem", "ne küldjön több", "ne írjanak többet",
		"ne keressenek többet", "ne keressenek a továbbiakban", "nem kérek több e-mail*", "nem kérek több level*",
	},
	"id": {
		"berhenti berlangganan", "hapus saya dari", "hapus email saya", "hapus data saya", "keluarkan saya dari",
		"jangan kirim email lagi", "jangan kirimi saya", "jangan hubungi saya", "tidak ingin menerima email",
	},
	"it": {
		"disiscrivermi", "disiscrivetemi", "cancellatemi dalla", "cancellami dalla", "cancellarmi dalla", "cancellate il mio indirizzo",
		"cancellate i miei dati", "rimuovetemi", "rimuovermi dalla", "rimuovete il mio indirizzo", "rimuovete la mia email", "toglietemi dalla", "non inviatemi più",
		"non mandatemi più", "smettete di inviarmi", "smettete di scrivermi", "non contattatemi più", "non voglio più ricevere",
		"non desidero più ricevere", "disiscrizione",
	},
	"ja": {
		"配信停止", "配信を停止", "配信不要", "配信解除", "購読解除", "登録解除", "メール不要", "今後のご連絡は不要",
		"今後の連絡は不要", "今後連絡しないでください", "もう連絡しないでください", "メールを送らないでください", "もう送らないでください",
		"今後送らないでください", "リストから削除", "リストから外して", "メールを停止",
	},
	"ko": {
		"수신거부", "수신 거부", "수신을 거부", "구독 취소", "구독취소", "구독 해지", "메일 보내지 마", "메일을 보내지 마",
		"이메일 보내지 마", "이메일을 보내지 마", "더 이상 보내지 마", "더 이상 연락하지 마", "다시 연락하지 마",
		"명단에서 삭제", "목록에서 삭제", "리스트에서 삭제", "제 정보를 삭제",
	},
	"lt": {
		"atsisakyti prenumeratos", "atsisakau prenumeratos", "išbraukite mane", "pašalinkite mane", "ištrinkite mane",
		"ištrinkite mano duomenis", "nebesiųskite", "nesiųskite man daugiau", "nebesikreipkite", "nebesusisiekite", "nebenoriu gauti",
	},
	"lv": {
		"atrakstīties", "atrakstiet mani", "atrakstīt mani", "izņemiet mani no", "izdzēsiet mani", "dzēsiet manus datus",
		"nesūtiet man vairs", "vairs nesūtiet", "pārtrauciet sūtīt", "nesazinieties ar mani", "vairs nerakstiet",
	},
	"ms": {
		"berhenti melanggan", "nyahlanggan", "buang saya dari", "keluarkan saya dari", "padamkan saya", "padam data saya",
		"jangan hantar lagi", "jangan hantar e-mel", "jangan hantar emel", "jangan hubungi saya", "tidak mahu menerima",
	},
	"nb": {
		"avmeld meg", "meld meg av listen", "meld meg ut av", "fjern meg fra", "slett meg", "slett mine opplysninger",
		"slutt å sende", "ikke send flere", "ikke flere e-poster", "ikke flere mail", "ikke kontakt meg", "ønsker ikke å motta",
	},
	"nl": {
		"afmelden van", "graag afmelden", "meld mij af van", "meld me af van", "uitschrijven", "schrijf mij uit", "schrijf me uit",
		"haal mij van", "haal me van", "verwijder mij", "verwijder me", "verwijder mijn gegevens", "verwijder mijn e-mailadres",
		"geen e-mails meer", "geen mails meer", "stuur mij geen mails", "stuur me geen mails", "stuur mij geen e-mails",
		"stuur me geen e-mails", "stuur me geen berichten meer", "neem geen contact meer", "niet meer benaderen",
		"niet meer mailen", "stop met mailen",
	},
	"pl": {
		"wypisz mnie", "wypiszcie mnie", "proszę mnie wypisać", "wypisać mnie", "usuńcie mnie", "usuń mnie", "proszę mnie usunąć",
		"usunąć mnie z", "usuńcie moje dane", "usunięcie moich danych", "proszę nie wysyłać więcej", "proszę nie wysyłać mi",
		"nie wysyłajcie mi", "nie wysyłaj mi więcej", "proszę się ze mną nie kontaktować", "nie kontaktujcie się ze mną", "rezygnuję z subskrypcji", "wypisanie z listy",
	},
	"pt": {
		"descadastrar", "descadastre", "me descadastrar", "cancelar a inscrição", "cancelar minha inscrição", "cancelar a subscrição",
		"remover da lista", "remova meu e-mail", "remova o meu e-mail", "remova-me", "me remova", "retire-me", "me retire da",
		"me tire da lista", "não me envie mais", "não me enviem mais", "parem de me enviar", "pare de me enviar",
		"não quero receber mais", "não desejo receber mais", "não me contate mais", "não me contactem mais",
		"apaguem os meus dados", "apague meus dados", "excluam meus dados",
	},
	"ro": {
		"dezabonare", "dezabonați-mă", "dezabonează-mă", "mă dezabonez", "scoateți-mă de pe listă", "scoateți-mă din",
		"eliminați-mă", "ștergeți-mă", "ștergeți datele mele", "nu-mi mai trimiteți", "nu mai trimiteți", "nu mă mai contactați",
		"opriți trimiterea",
	},
	"ru": {
		"отпишите меня", "отпишите нас", "отписаться", "удалите меня", "удалите мой адрес", "удалите мои данные",
		"исключите меня из", "уберите меня из", "не пишите мне больше", "не присылайте мне больше", "не присылайте больше",
		"больше не пишите", "больше не присылайте", "прекратите рассылку", "прекратите присылать", "не связывайтесь со мной",
		"не беспокойте меня больше",
	},
	"sk": {
		"odhláste ma", "odhlásiť ma", "odhlásiť z odberu", "zrušiť odber", "zrušte odber", "odstráňte ma", "vymažte ma",
		"vymažte moje údaje", "neposielajte mi už", "neposielajte mi ďalšie", "prestaňte mi posielať", "nekontaktujte ma", "nechcem dostávať",
	},
	"sl": {
		"odjavite me", "odjava od prejemanja", "odstranite me", "izbrišite me", "izbrišite moje podatke", "ne pošiljajte mi več",
		"nehajte mi pošiljati", "ne kontaktirajte me", "ne pišite mi več",
	},
	"sr": {
		"одјавите ме", "уклоните ме", "избришите ме", "обришите моје податке", "не шаљите ми више", "престаните да ми шаљете",
		"не контактирајте ме", "не пишите ми више",
		"odjavite me", "uklonite me", "izbrišite me", "obrišite moje podatke", "ne šaljite mi više", "prestanite da mi šaljete",
		"ne kontaktirajte me", "ne pišite mi više",
	},
	"sv": {
		"avregistrera mig", "avsluta prenumerationen", "avprenumerera", "ta bort mig", "ta bort min e-post", "ta bort mina uppgifter",
		"sluta skicka", "sluta mejla", "skicka inga fler", "inga fler mejl", "inga fler mail", "kontakta mig inte",
		"kontakta oss inte", "vill inte få fler",
	},
	"sw": {
		"niondoe kwenye orodha", "niondoe", "niondoeni", "acha kunitumia", "acheni kunitumia", "usinitumie", "msinitumie",
		"usiwasiliane nami", "sitaki kupokea", "futa taarifa zangu",
	},
	"ta": {
		"குழுவிலகு*", "சந்தாவை ரத்து*", "என்னை பட்டியலிலிருந்து நீக்க*", "என்னை நீக்க*", "மின்னஞ்சல் அனுப்ப வேண்டாம்",
		"இனி அனுப்ப வேண்டாம்", "என்னை தொடர்பு கொள்ள வேண்டாம்", "எனது தகவல்களை நீக்க*",
	},
	"th": {
		"ยกเลิกการสมัคร", "ยกเลิกการรับข่าวสาร", "ลบฉันออกจาก", "ลบผมออกจาก", "ลบอีเมลของฉัน", "ไม่ต้องส่งมาอีก",
		"อย่าส่งมาอีก", "หยุดส่งอีเมล", "ไม่ต้องติดต่อมาอีก", "ไม่ต้องการรับอีเมล",
	},
	"tr": {
		"abonelikten çık*", "listeden çıkar*", "listenizden çıkar*", "beni listeden", "beni sil*", "beni çıkar*",
		"mail göndermeyin", "e-posta göndermeyin", "bana mail atmayın", "bana e-posta göndermeyin", "benimle iletişime geçmeyin",
		"bir daha yazmayın", "verilerimi sil*",
	},
	"uk": {
		"відпишіть мене", "відписатися", "видаліть мене", "видаліть мою адресу", "видаліть мої дані", "не пишіть мені більше",
		"не надсилайте мені більше", "більше не пишіть", "більше не надсилайте", "припиніть розсилку", "не турбуйте мене більше",
		"не контактуйте зі мною",
	},
	"ur": {
		"ان سبسکرائب", "سبسکرپشن منسوخ", "مجھے فہرست سے نکال*", "مجھے ہٹا دیں", "مجھے نکال دیں", "ای میل نہ بھیجیں",
		"ای میل مت بھیجیں", "مزید ای میل نہ", "مجھ سے رابطہ نہ کریں", "میرا ڈیٹا حذف",
	},
	"vi": {
		"hủy đăng ký", "huỷ đăng ký", "xóa tôi khỏi", "xoá tôi khỏi", "gỡ tôi khỏi", "đừng gửi email", "đừng gửi thư",
		"ngừng gửi email", "ngừng gửi thư", "không gửi email cho tôi nữa", "đừng liên hệ với tôi", "xóa thông tin của tôi",
		"không muốn nhận email",
	},
	"zh": {
		"取消订阅", "取消訂閱", "退订", "退訂", "不要再发", "不要再發", "别再发", "別再發", "请勿再发", "請勿再發",
		"从列表中删除", "從名單中刪除", "从名单中删除", "把我从", "把我從", "删除我的邮箱", "删除我的信息", "删除我的资料",
		"刪除我的資料", "不要再联系我", "不要再聯絡我", "别再联系我", "別再聯絡我", "请不要再给我发", "請不要再寄",
	},
}

// optOutLanguages are the languages the opt-out phrases are in. Opt-out is
// read without the workspace's tagging languages, yet a campaign written in
// one of these carries its own opt-out line in that language, and the people
// who answer it quote it with their mail clients' markers ("Von: ...
// Gesendet: ..."). Those markers are always cut first, or every such reply
// would opt its sender out.
var optOutLanguages = func() []string {
	out := []string{"de"}
	for code := range optOutPhrases {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}()

var optOutPatterns = compileWordPatterns(allOptOutPhrases())

func allOptOutPhrases() []string {
	out := append([]string(nil), unsubscribeKeywords...)
	for _, code := range optOutLanguages {
		out = append(out, optOutPhrases[code]...)
	}
	return out
}

// compileWordPatterns anchors each phrase on word boundaries in any script;
// "don't" and "opt-out" keep their apostrophe and hyphen literal. Scripts
// written without spaces between words, and an end marked "*", stay open.
func compileWordPatterns(phrases []string) []*regexp.Regexp {
	const boundary = `[^\pL\pM\pN]`
	out := make([]*regexp.Regexp, 0, len(phrases))
	for _, p := range phrases {
		openStart, openEnd := strings.HasPrefix(p, "*"), strings.HasSuffix(p, "*")
		p = foldOptOut(strings.Trim(p, "*"))
		first, last := []rune(p)[0], []rune(p)[len([]rune(p))-1]
		re := regexp.QuoteMeta(p)
		if !openStart && !unspaced(first) {
			re = `(^|` + boundary + `)` + re
		}
		if !openEnd && !unspaced(last) {
			re += `($|` + boundary + `)`
		}
		out = append(out, regexp.MustCompile(re))
	}
	return out
}

// unspaced is a letter of a script that runs its words together, where a
// phrase has no boundary to stand on.
func unspaced(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Thai)
}

// optOutFolder evens out spellings a phrase list would otherwise need twice:
// Greek's final sigma, the "đ" of Vietnamese typed without marks, and the
// Arabic letters Persian and Urdu keyboards type in their own form.
var optOutFolder = strings.NewReplacer("ς", "σ", "đ", "d", "أ", "ا", "إ", "ا", "آ", "ا", "ي", "ی", "ك", "ک", "ى", "ی")

// foldOptOut is what both a phrase and a reply are matched as: lower case,
// accents flattened (the Latin and Greek marks accentFolder has no entry for
// are dropped), single spaces.
func foldOptOut(s string) string {
	s = accentFolder.Replace(strings.ToLower(norm.NFC.String(s)))
	var b strings.Builder
	strip := false
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			if strip {
				continue
			}
		} else {
			strip = unicode.In(r, unicode.Latin, unicode.Greek)
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(optOutFolder.Replace(norm.NFC.String(b.String()))), " ")
}
