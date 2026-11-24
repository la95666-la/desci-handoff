package i18n

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"joincare_server/pkg/common/utils"
)

type I18nLang string

const (
	EN    I18nLang = "en"
	EN_US I18nLang = "en-us"
	EN_UK I18nLang = "en-uk"
	ZH    I18nLang = "zh"
	ZH_CN I18nLang = "zh-cn"
	ZH_TW I18nLang = "zh-tw"

	format = `{{%d}}`
)

var (
	//def, all = []I18nLang{ZH}, []I18nLang{ZH_CN} // just for now
	def, all = []I18nLang{ZH, EN}, []I18nLang{ZH_CN, ZH_TW, EN_US}

	data = make(map[I18nLang]map[string]string, len(all))
)

func I18nInit(path string) {
	storage := func(file string) {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			box := make(map[string]string, 300)
			read(file, func(line string) {
				if i := strings.Index(line, "="); i > 0 && !strings.HasPrefix(line, "#") {
					box[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
				}
			})

			lang := I18nLang(file[strings.LastIndex(file, string(os.PathSeparator))+1 : strings.LastIndex(file, ".")])
			data[lang] = box
			fmt.Printf("Read i18n(%s) successfully\n", lang)
		} else {
			fmt.Println(file + " not found")
		}
	}

	for _, lang := range all {
		storage(filepath.Join(path, string(lang)+".lang"))
	}
}

func Localize(lang I18nLang, key string, a []any) string {
	str := ""
	l, ok := data[lang]
	if !ok {
		return ""
	}
	v, has := l[key]
	if !has {
		return ""
	}
	str = v
	if tot := len(a); tot > 0 {
		for i := 0; i < tot; i++ {
			str = strings.Replace(str, fmt.Sprintf(format, i+1), utils.Any2String((a)[i]), 1)
		}
	}
	return str
}

func FindLang(lang string) I18nLang {
	lang = strings.ToLower(lang)
	lang = strings.ReplaceAll(lang, "_", "-")
	for _, l := range all {
		if lang == string(l) {

			return l
		}
	}

	d := strings.Split(lang, "-")[0]
	for _, l := range def {
		if d == string(l) {
			switch l {
			case ZH:
				return ZH_TW
			case EN:
				return EN_US
			}
		}
	}

	return ZH_TW
}

func read(path string, todo func(line string)) {

	if file, err := os.Open(path); err == nil {

		defer file.Close()

		r := bufio.NewReader(file)
		for {

			bytes, _, err := r.ReadLine()
			if err == nil {

				todo(strings.TrimSpace(string(bytes)))
			} else {

				if err == io.EOF {
					break
				}
				panic(err)
			}
		}
	} else {
		panic(err)
	}
}
