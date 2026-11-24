package utils

import (
	"sort"
	"strings"

	"github.com/spf13/cast"
)

func MkNavString(dataMap *map[string]any) string {
	data := *dataMap
	if tot := len(data); tot > 0 {
		list := make([]string, 0, tot)
		for k := range data {
			list = append(list, k)
		}
		sort.Strings(list)

		for i, k := range list {
			list[i] += ">" + Any2String(data[k])
		}
		return strings.Join(list, "~~")
	}

	return ""
}

func Any2String(obj any) (v string) {
	if obj == nil {
		v = ""
	} else {
		switch obj := obj.(type) {
		case string:
			v = obj
		case []any:
			v = array2String(&obj)
		case map[string]any:
			v = MkNavString(&obj)
		default:
			v = cast.ToString(obj)
		}
	}

	return
}

func array2String(arr *[]any) string {
	strs := make([]string, len(*arr))
	for i, v := range *arr {
		strs[i] = Any2String(v)
	}
	sort.Strings(strs)
	return strings.Join(strs, "|")
}
