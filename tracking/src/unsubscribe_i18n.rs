//! The tracking service's own unsubscribe pages, in the recipient's language.
//! The wording matches the backend's (internal/api/handler/unsubscribe_i18n.go),
//! so a probe cannot tell an invalid-shaped token from an invalid one.

/// One language's copy of the two pages the proxy answers itself.
pub struct PageCopy {
    pub rtl: bool,
    pub invalid_title: &'static str,
    pub reply_body: &'static str,
    pub retry_title: &'static str,
    pub unavailable_body: &'static str,
}

/// The copies by BCP 47 code, English first.
pub const COPIES: &[(&str, PageCopy)] = &[
    (
        "en",
        PageCopy {
            rtl: false,
            invalid_title: "This unsubscribe link is invalid",
            reply_body: "Reply to the email instead and the sender will stop.",
            retry_title: "Try again shortly",
            unavailable_body: "We could not reach the sender's server just now. Open this link again in a few minutes, or reply to the email and the sender will stop.",
        },
    ),
    (
        "ar",
        PageCopy {
            rtl: true,
            invalid_title: "رابط إلغاء الاشتراك هذا غير صالح",
            reply_body: "يمكنك بدلًا من ذلك الرد على الرسالة وسيتوقف المرسِل عن مراسلتك.",
            retry_title: "حاول مرة أخرى بعد قليل",
            unavailable_body: "تعذّر علينا الوصول إلى خادم المرسِل الآن. افتح هذا الرابط مرة أخرى بعد بضع دقائق، أو ردّ على الرسالة وسيتوقف المرسِل عن مراسلتك.",
        },
    ),
    (
        "bg",
        PageCopy {
            rtl: false,
            invalid_title: "Тази връзка за отписване е невалидна",
            reply_body: "Вместо това отговорете на имейла и подателят ще спре да ви пише.",
            retry_title: "Опитайте отново след малко",
            unavailable_body: "В момента не успяхме да се свържем със сървъра на подателя. Отворете тази връзка отново след няколко минути или отговорете на имейла и подателят ще спре да ви пише.",
        },
    ),
    (
        "bn",
        PageCopy {
            rtl: false,
            invalid_title: "এই আনসাবস্ক্রাইব লিংকটি বৈধ নয়",
            reply_body: "এর পরিবর্তে ইমেইলটির উত্তর দিন, তাহলে প্রেরক আপনাকে ইমেইল পাঠানো বন্ধ করবেন।",
            retry_title: "কিছুক্ষণ পরে আবার চেষ্টা করুন",
            unavailable_body: "এই মুহূর্তে আমরা প্রেরকের সার্ভারে পৌঁছাতে পারিনি। কয়েক মিনিট পরে এই লিংকটি আবার খুলুন, অথবা ইমেইলটির উত্তর দিন, তাহলে প্রেরক আপনাকে ইমেইল পাঠানো বন্ধ করবেন।",
        },
    ),
    (
        "ca",
        PageCopy {
            rtl: false,
            invalid_title: "Aquest enllaç per cancel·lar la subscripció no és vàlid",
            reply_body: "En lloc d'això, responeu al correu i el remitent deixarà d'escriure-us.",
            retry_title: "Torneu-ho a provar d'aquí a una estona",
            unavailable_body: "Ara mateix no hem pogut contactar amb el servidor del remitent. Torneu a obrir aquest enllaç d'aquí a uns minuts o responeu al correu i el remitent deixarà d'escriure-us.",
        },
    ),
    (
        "cs",
        PageCopy {
            rtl: false,
            invalid_title: "Tento odkaz k odhlášení je neplatný",
            reply_body: "Odpovězte místo toho na e-mail a odesílatel vám přestane psát.",
            retry_title: "Zkuste to prosím za chvíli",
            unavailable_body: "Server odesílatele se nám teď nepodařilo kontaktovat. Otevřete tento odkaz znovu za několik minut, nebo odpovězte na e-mail a odesílatel vám přestane psát.",
        },
    ),
    (
        "da",
        PageCopy {
            rtl: false,
            invalid_title: "Afmeldingslinket er ugyldigt",
            reply_body: "Svar i stedet på e-mailen, så holder afsenderen op med at skrive til dig.",
            retry_title: "Prøv igen om lidt",
            unavailable_body: "Vi kunne ikke få kontakt til afsenderens server lige nu. Åbn linket igen om et par minutter, eller svar på e-mailen, så holder afsenderen op med at skrive til dig.",
        },
    ),
    (
        "de",
        PageCopy {
            rtl: false,
            invalid_title: "Dieser Abmeldelink ist ungültig",
            reply_body: "Antworten Sie stattdessen auf die E-Mail, dann schreibt Ihnen der Absender nicht mehr.",
            retry_title: "Bitte versuchen Sie es gleich noch einmal",
            unavailable_body: "Wir konnten den Server des Absenders gerade nicht erreichen. Öffnen Sie diesen Link in ein paar Minuten erneut oder antworten Sie auf die E-Mail, dann schreibt Ihnen der Absender nicht mehr.",
        },
    ),
    (
        "el",
        PageCopy {
            rtl: false,
            invalid_title: "Αυτός ο σύνδεσμος κατάργησης εγγραφής δεν είναι έγκυρος",
            reply_body: "Εναλλακτικά, απαντήστε στο email και ο αποστολέας θα σταματήσει να σας γράφει.",
            retry_title: "Δοκιμάστε ξανά σε λίγο",
            unavailable_body: "Δεν ήταν δυνατή η σύνδεση με τον διακομιστή του αποστολέα αυτή τη στιγμή. Ανοίξτε ξανά αυτόν τον σύνδεσμο σε λίγα λεπτά ή απαντήστε στο email και ο αποστολέας θα σταματήσει να σας γράφει.",
        },
    ),
    (
        "es",
        PageCopy {
            rtl: false,
            invalid_title: "Este enlace para darse de baja no es válido",
            reply_body: "En su lugar, responda al correo y el remitente dejará de escribirle.",
            retry_title: "Vuelva a intentarlo en unos momentos",
            unavailable_body: "No hemos podido conectar con el servidor del remitente en este momento. Abra este enlace de nuevo en unos minutos o responda al correo y el remitente dejará de escribirle.",
        },
    ),
    (
        "et",
        PageCopy {
            rtl: false,
            invalid_title: "See tellimusest loobumise link on kehtetu",
            reply_body: "Vastake selle asemel kirjale ja saatja lõpetab teile kirjutamise.",
            retry_title: "Proovige veidi aja pärast uuesti",
            unavailable_body: "Me ei saanud praegu saatja serveriga ühendust. Avage see link mõne minuti pärast uuesti või vastake kirjale ja saatja lõpetab teile kirjutamise.",
        },
    ),
    (
        "fa",
        PageCopy {
            rtl: true,
            invalid_title: "این پیوند لغو اشتراک نامعتبر است",
            reply_body: "به‌جای آن به ایمیل پاسخ دهید تا فرستنده دیگر برای شما ایمیل نفرستد.",
            retry_title: "کمی بعد دوباره امتحان کنید",
            unavailable_body: "در حال حاضر نتوانستیم به سرور فرستنده دسترسی پیدا کنیم. چند دقیقه دیگر دوباره این پیوند را باز کنید، یا به ایمیل پاسخ دهید تا فرستنده دیگر برای شما ایمیل نفرستد.",
        },
    ),
    (
        "fi",
        PageCopy {
            rtl: false,
            invalid_title: "Tilauksen peruutuslinkki ei ole kelvollinen",
            reply_body: "Vastaa sen sijaan sähköpostiin, niin lähettäjä lopettaa viestien lähettämisen.",
            retry_title: "Yritä hetken kuluttua uudelleen",
            unavailable_body: "Emme saaneet juuri nyt yhteyttä lähettäjän palvelimeen. Avaa linkki uudelleen muutaman minuutin kuluttua tai vastaa sähköpostiin, niin lähettäjä lopettaa viestien lähettämisen.",
        },
    ),
    (
        "fil",
        PageCopy {
            rtl: false,
            invalid_title: "Hindi valid ang unsubscribe link na ito",
            reply_body: "Sa halip, sagutin ang email at titigil na ang nagpadala sa pag-email sa inyo.",
            retry_title: "Subukang muli maya-maya",
            unavailable_body: "Hindi namin maabot ang server ng nagpadala sa ngayon. Buksan muli ang link na ito pagkalipas ng ilang minuto, o sagutin ang email at titigil na ang nagpadala sa pag-email sa inyo.",
        },
    ),
    (
        "fr",
        PageCopy {
            rtl: false,
            invalid_title: "Ce lien de désabonnement n'est pas valide",
            reply_body: "Répondez plutôt à l'e-mail et l'expéditeur cessera de vous écrire.",
            retry_title: "Réessayez dans quelques instants",
            unavailable_body: "Nous n'avons pas pu joindre le serveur de l'expéditeur pour le moment. Ouvrez à nouveau ce lien dans quelques minutes, ou répondez à l'e-mail et l'expéditeur cessera de vous écrire.",
        },
    ),
    (
        "he",
        PageCopy {
            rtl: true,
            invalid_title: "קישור ביטול ההרשמה אינו תקין",
            reply_body: "במקום זאת, אפשר להשיב להודעה והשולח יפסיק לשלוח אליך הודעות.",
            retry_title: "נא לנסות שוב בעוד כמה רגעים",
            unavailable_body: "לא הצלחנו להתחבר לשרת של השולח כרגע. אפשר לפתוח את הקישור שוב בעוד כמה דקות, או להשיב להודעה והשולח יפסיק לשלוח אליך הודעות.",
        },
    ),
    (
        "hi",
        PageCopy {
            rtl: false,
            invalid_title: "सदस्यता समाप्त करने का यह लिंक अमान्य है",
            reply_body: "इसके बजाय ईमेल का जवाब दें, तो प्रेषक आपको ईमेल भेजना बंद कर देगा।",
            retry_title: "थोड़ी देर में फिर से प्रयास करें",
            unavailable_body: "हम अभी प्रेषक के सर्वर तक नहीं पहुँच सके। कुछ मिनट बाद यह लिंक फिर से खोलें, या ईमेल का जवाब दें, तो प्रेषक आपको ईमेल भेजना बंद कर देगा।",
        },
    ),
    (
        "hr",
        PageCopy {
            rtl: false,
            invalid_title: "Ova poveznica za odjavu nije valjana",
            reply_body: "Umjesto toga odgovorite na e-poruku i pošiljatelj vam više neće pisati.",
            retry_title: "Pokušajte ponovno za koji trenutak",
            unavailable_body: "Trenutačno se nismo mogli povezati s poslužiteljem pošiljatelja. Otvorite ovu poveznicu ponovno za nekoliko minuta ili odgovorite na e-poruku i pošiljatelj vam više neće pisati.",
        },
    ),
    (
        "hu",
        PageCopy {
            rtl: false,
            invalid_title: "Ez a leiratkozási hivatkozás érvénytelen",
            reply_body: "Válaszoljon inkább az e-mailre, és a feladó nem ír Önnek többet.",
            retry_title: "Próbálja újra kicsit később",
            unavailable_body: "A feladó szerverét most nem sikerült elérnünk. Nyissa meg újra ezt a hivatkozást néhány perc múlva, vagy válaszoljon az e-mailre, és a feladó nem ír Önnek többet.",
        },
    ),
    (
        "id",
        PageCopy {
            rtl: false,
            invalid_title: "Tautan berhenti berlangganan ini tidak valid",
            reply_body: "Sebagai gantinya, balas email tersebut dan pengirim akan berhenti mengirimi Anda email.",
            retry_title: "Coba lagi sebentar lagi",
            unavailable_body: "Kami tidak dapat menghubungi server pengirim saat ini. Buka kembali tautan ini dalam beberapa menit, atau balas email tersebut dan pengirim akan berhenti mengirimi Anda email.",
        },
    ),
    (
        "it",
        PageCopy {
            rtl: false,
            invalid_title: "Questo link di annullamento dell'iscrizione non è valido",
            reply_body: "Risponda invece all'email e il mittente smetterà di scriverle.",
            retry_title: "Riprovi tra poco",
            unavailable_body: "Al momento non è stato possibile raggiungere il server del mittente. Riapra questo link tra qualche minuto, oppure risponda all'email e il mittente smetterà di scriverle.",
        },
    ),
    (
        "ja",
        PageCopy {
            rtl: false,
            invalid_title: "この配信停止リンクは無効です",
            reply_body: "代わりにメールへ返信していただければ、送信者は送信を停止します。",
            retry_title: "しばらくしてからもう一度お試しください",
            unavailable_body: "現在、送信者のサーバーに接続できませんでした。数分後にこのリンクをもう一度開くか、メールへ返信していただければ、送信者は送信を停止します。",
        },
    ),
    (
        "ko",
        PageCopy {
            rtl: false,
            invalid_title: "유효하지 않은 수신 거부 링크입니다",
            reply_body: "대신 이메일에 회신하시면 발신자가 발송을 중단합니다.",
            retry_title: "잠시 후 다시 시도해 주세요",
            unavailable_body: "지금은 발신자의 서버에 연결할 수 없습니다. 몇 분 후 이 링크를 다시 열어 보시거나, 이메일에 회신하시면 발신자가 발송을 중단합니다.",
        },
    ),
    (
        "lt",
        PageCopy {
            rtl: false,
            invalid_title: "Ši prenumeratos atsisakymo nuoroda negalioja",
            reply_body: "Vietoj to atsakykite į el. laišką ir siuntėjas jums nebesiųs laiškų.",
            retry_title: "Netrukus bandykite dar kartą",
            unavailable_body: "Šiuo metu nepavyko susisiekti su siuntėjo serveriu. Atidarykite šią nuorodą dar kartą po kelių minučių arba atsakykite į el. laišką ir siuntėjas jums nebesiųs laiškų.",
        },
    ),
    (
        "lv",
        PageCopy {
            rtl: false,
            invalid_title: "Šī atteikšanās saite nav derīga",
            reply_body: "Tā vietā atbildiet uz e-pastu, un sūtītājs jums vairs nerakstīs.",
            retry_title: "Mēģiniet vēlreiz nedaudz vēlāk",
            unavailable_body: "Pašlaik neizdevās sazināties ar sūtītāja serveri. Atveriet šo saiti vēlreiz pēc dažām minūtēm vai atbildiet uz e-pastu, un sūtītājs jums vairs nerakstīs.",
        },
    ),
    (
        "ms",
        PageCopy {
            rtl: false,
            invalid_title: "Pautan berhenti melanggan ini tidak sah",
            reply_body: "Sebaliknya, balas e-mel tersebut dan pengirim akan berhenti menghantar e-mel kepada anda.",
            retry_title: "Cuba lagi sebentar nanti",
            unavailable_body: "Kami tidak dapat menghubungi pelayan pengirim sekarang. Buka semula pautan ini dalam beberapa minit, atau balas e-mel tersebut dan pengirim akan berhenti menghantar e-mel kepada anda.",
        },
    ),
    (
        "nb",
        PageCopy {
            rtl: false,
            invalid_title: "Denne avmeldingslenken er ugyldig",
            reply_body: "Svar på e-posten i stedet, så slutter avsenderen å sende deg e-post.",
            retry_title: "Prøv igjen om litt",
            unavailable_body: "Vi fikk ikke kontakt med avsenderens server akkurat nå. Åpne denne lenken igjen om noen minutter, eller svar på e-posten, så slutter avsenderen å sende deg e-post.",
        },
    ),
    (
        "nl",
        PageCopy {
            rtl: false,
            invalid_title: "Deze afmeldlink is ongeldig",
            reply_body: "Beantwoord in plaats daarvan de e-mail, dan mailt de afzender u niet meer.",
            retry_title: "Probeer het zo opnieuw",
            unavailable_body: "We konden de server van de afzender nu niet bereiken. Open deze link over een paar minuten opnieuw, of beantwoord de e-mail, dan mailt de afzender u niet meer.",
        },
    ),
    (
        "pl",
        PageCopy {
            rtl: false,
            invalid_title: "Ten link do wypisania jest nieprawidłowy",
            reply_body: "Zamiast tego wystarczy odpowiedzieć na wiadomość, a nadawca przestanie pisać.",
            retry_title: "Prosimy spróbować ponownie za chwilę",
            unavailable_body: "Nie udało się teraz połączyć z serwerem nadawcy. Prosimy otworzyć ten link ponownie za kilka minut lub odpowiedzieć na wiadomość, a nadawca przestanie pisać.",
        },
    ),
    (
        "pt",
        PageCopy {
            rtl: false,
            invalid_title: "Este link de cancelamento é inválido",
            reply_body: "Em vez disso, responda ao e-mail e o remetente deixará de enviar mensagens.",
            retry_title: "Tente novamente em instantes",
            unavailable_body: "Não foi possível contatar o servidor do remetente agora. Abra este link novamente em alguns minutos ou responda ao e-mail e o remetente deixará de enviar mensagens.",
        },
    ),
    (
        "ro",
        PageCopy {
            rtl: false,
            invalid_title: "Acest link de dezabonare nu este valid",
            reply_body: "Răspundeți în schimb la e-mail, iar expeditorul nu vă va mai scrie.",
            retry_title: "Încercați din nou în curând",
            unavailable_body: "Nu am putut contacta serverul expeditorului acum. Deschideți din nou acest link peste câteva minute sau răspundeți la e-mail, iar expeditorul nu vă va mai scrie.",
        },
    ),
    (
        "ru",
        PageCopy {
            rtl: false,
            invalid_title: "Эта ссылка для отписки недействительна",
            reply_body: "Вместо этого ответьте на письмо, и отправитель перестанет вам писать.",
            retry_title: "Попробуйте чуть позже",
            unavailable_body: "Сейчас не удалось связаться с сервером отправителя. Откройте эту ссылку снова через несколько минут или ответьте на письмо, и отправитель перестанет вам писать.",
        },
    ),
    (
        "sk",
        PageCopy {
            rtl: false,
            invalid_title: "Tento odkaz na odhlásenie je neplatný",
            reply_body: "Namiesto toho odpovedzte na e-mail a odosielateľ vám prestane písať.",
            retry_title: "Skúste to o chvíľu znova",
            unavailable_body: "Server odosielateľa sa teraz nepodarilo kontaktovať. Otvorte tento odkaz znova o niekoľko minút alebo odpovedzte na e-mail a odosielateľ vám prestane písať.",
        },
    ),
    (
        "sl",
        PageCopy {
            rtl: false,
            invalid_title: "Ta povezava za odjavo ni veljavna",
            reply_body: "Namesto tega odgovorite na e-sporočilo in pošiljatelj vam ne bo več pisal.",
            retry_title: "Poskusite znova čez nekaj trenutkov",
            unavailable_body: "Strežnika pošiljatelja trenutno ni bilo mogoče doseči. To povezavo znova odprite čez nekaj minut ali odgovorite na e-sporočilo in pošiljatelj vam ne bo več pisal.",
        },
    ),
    (
        "sr",
        PageCopy {
            rtl: false,
            invalid_title: "Овај линк за одјаву није важећи",
            reply_body: "Уместо тога одговорите на поруку и пошиљалац ће престати да вам пише.",
            retry_title: "Покушајте поново ускоро",
            unavailable_body: "Тренутно нисмо могли да успоставимо везу са сервером пошиљаоца. Отворите овај линк поново за неколико минута или одговорите на поруку и пошиљалац ће престати да вам пише.",
        },
    ),
    (
        "sv",
        PageCopy {
            rtl: false,
            invalid_title: "Länken för att avsluta prenumerationen är ogiltig",
            reply_body: "Svara på mejlet i stället så slutar avsändaren att skriva till dig.",
            retry_title: "Försök igen om en stund",
            unavailable_body: "Vi kunde inte nå avsändarens server just nu. Öppna länken igen om några minuter, eller svara på mejlet så slutar avsändaren att skriva till dig.",
        },
    ),
    (
        "sw",
        PageCopy {
            rtl: false,
            invalid_title: "Kiungo hiki cha kujiondoa si halali",
            reply_body: "Badala yake, jibu barua pepe hiyo na mtumaji ataacha kukutumia barua.",
            retry_title: "Jaribu tena baada ya muda mfupi",
            unavailable_body: "Hatukuweza kufikia seva ya mtumaji kwa sasa. Fungua kiungo hiki tena baada ya dakika chache, au jibu barua pepe hiyo na mtumaji ataacha kukutumia barua.",
        },
    ),
    (
        "ta",
        PageCopy {
            rtl: false,
            invalid_title: "இந்தக் குழுவிலகல் இணைப்பு செல்லாது",
            reply_body: "அதற்குப் பதிலாக மின்னஞ்சலுக்குப் பதிலளியுங்கள், அனுப்புநர் உங்களுக்கு அனுப்புவதை நிறுத்துவார்.",
            retry_title: "சிறிது நேரத்தில் மீண்டும் முயலுங்கள்",
            unavailable_body: "இப்போது அனுப்புநரின் சேவையகத்தை அணுக முடியவில்லை. சில நிமிடங்களில் இந்த இணைப்பை மீண்டும் திறக்கவும், அல்லது மின்னஞ்சலுக்குப் பதிலளியுங்கள், அனுப்புநர் உங்களுக்கு அனுப்புவதை நிறுத்துவார்.",
        },
    ),
    (
        "th",
        PageCopy {
            rtl: false,
            invalid_title: "ลิงก์ยกเลิกการรับอีเมลนี้ไม่ถูกต้อง",
            reply_body: "โปรดตอบกลับอีเมลแทน แล้วผู้ส่งจะหยุดส่งอีเมลถึงคุณ",
            retry_title: "โปรดลองอีกครั้งในอีกสักครู่",
            unavailable_body: "ขณะนี้ไม่สามารถเชื่อมต่อกับเซิร์ฟเวอร์ของผู้ส่งได้ โปรดเปิดลิงก์นี้อีกครั้งในอีกไม่กี่นาที หรือตอบกลับอีเมล แล้วผู้ส่งจะหยุดส่งอีเมลถึงคุณ",
        },
    ),
    (
        "tr",
        PageCopy {
            rtl: false,
            invalid_title: "Bu abonelikten çıkma bağlantısı geçersiz",
            reply_body: "Bunun yerine e-postayı yanıtlayın, gönderici size yazmayı bırakacaktır.",
            retry_title: "Birazdan tekrar deneyin",
            unavailable_body: "Göndericinin sunucusuna şu anda ulaşamadık. Bu bağlantıyı birkaç dakika sonra yeniden açın veya e-postayı yanıtlayın, gönderici size yazmayı bırakacaktır.",
        },
    ),
    (
        "uk",
        PageCopy {
            rtl: false,
            invalid_title: "Це посилання для відписки недійсне",
            reply_body: "Натомість дайте відповідь на лист, і відправник перестане вам писати.",
            retry_title: "Спробуйте трохи згодом",
            unavailable_body: "Зараз не вдалося зв'язатися із сервером відправника. Відкрийте це посилання знову за кілька хвилин або дайте відповідь на лист, і відправник перестане вам писати.",
        },
    ),
    (
        "ur",
        PageCopy {
            rtl: true,
            invalid_title: "یہ ان سبسکرائب لنک درست نہیں ہے",
            reply_body: "اس کے بجائے ای میل کا جواب دیں، بھیجنے والا آپ کو ای میل بھیجنا بند کر دے گا۔",
            retry_title: "تھوڑی دیر میں دوبارہ کوشش کریں",
            unavailable_body: "ہم ابھی بھیجنے والے کے سرور تک نہیں پہنچ سکے۔ چند منٹ بعد یہ لنک دوبارہ کھولیں، یا ای میل کا جواب دیں، بھیجنے والا آپ کو ای میل بھیجنا بند کر دے گا۔",
        },
    ),
    (
        "vi",
        PageCopy {
            rtl: false,
            invalid_title: "Liên kết hủy đăng ký này không hợp lệ",
            reply_body: "Thay vào đó, hãy trả lời email và người gửi sẽ ngừng gửi cho bạn.",
            retry_title: "Vui lòng thử lại sau ít phút",
            unavailable_body: "Hiện chúng tôi chưa thể kết nối tới máy chủ của người gửi. Hãy mở lại liên kết này sau vài phút, hoặc trả lời email và người gửi sẽ ngừng gửi cho bạn.",
        },
    ),
    (
        "zh",
        PageCopy {
            rtl: false,
            invalid_title: "此退订链接无效",
            reply_body: "请直接回复邮件，发件人将不再给您发送邮件。",
            retry_title: "请稍后重试",
            unavailable_body: "暂时无法连接到发件人的服务器。请几分钟后重新打开此链接，或回复邮件，发件人将不再给您发送邮件。",
        },
    ),
    (
        "zh-Hant",
        PageCopy {
            rtl: false,
            invalid_title: "此取消訂閱連結無效",
            reply_body: "請直接回覆郵件，寄件者就會停止寄信給您。",
            retry_title: "請稍後再試",
            unavailable_body: "目前無法連線到寄件者的伺服器。請在幾分鐘後重新開啟此連結，或回覆郵件，寄件者就會停止寄信給您。",
        },
    ),
];
