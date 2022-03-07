package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	json "encoding/json"

	easyjson "github.com/mailru/easyjson"

	jlexer "github.com/mailru/easyjson/jlexer"

	jwriter "github.com/mailru/easyjson/jwriter"
)

type User struct {
	Browsers []string `json:"browsers"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
}

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson8ceb9162DecodeFile(in *jlexer.Lexer, out *User) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "browsers":
			if in.IsNull() {
				in.Skip()
				out.Browsers = nil
			} else {
				in.Delim('[')
				if out.Browsers == nil {
					if !in.IsDelim(']') {
						out.Browsers = make([]string, 0, 4)
					} else {
						out.Browsers = []string{}
					}
				} else {
					out.Browsers = (out.Browsers)[:0]
				}
				for !in.IsDelim(']') {
					var v1 string
					v1 = string(in.String())
					out.Browsers = append(out.Browsers, v1)
					in.WantComma()
				}
				in.Delim(']')
			}
		case "email":
			out.Email = string(in.String())
		case "name":
			out.Name = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson8ceb9162EncodeFile(out *jwriter.Writer, in User) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"browsers\":"
		out.RawString(prefix[1:])
		if in.Browsers == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v2, v3 := range in.Browsers {
				if v2 > 0 {
					out.RawByte(',')
				}
				out.String(string(v3))
			}
			out.RawByte(']')
		}
	}
	{
		const prefix string = ",\"email\":"
		out.RawString(prefix)
		out.String(string(in.Email))
	}
	{
		const prefix string = ",\"name\":"
		out.RawString(prefix)
		out.String(string(in.Name))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v User) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson8ceb9162EncodeFile(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v User) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson8ceb9162EncodeFile(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *User) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson8ceb9162DecodeFile(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *User) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson8ceb9162DecodeFile(l, v)
}

func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	users := make([]User, 0, 1000)
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadSlice(10)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		user := User{}
		err = user.UnmarshalJSON(line)
		if err != nil {
			panic(err)
		}
		users = append(users, user)
	}

	seenBrowsers := map[string]bool{}

	io.WriteString(out, "found users:\n")

	for i, user := range users {
		isAndroid := false
		isMSIE := false

		for _, browser := range user.Browsers {
			if ok := strings.Contains(browser, "Android"); ok {
				isAndroid = true
				if _, ok := seenBrowsers[browser]; !ok {
					seenBrowsers[browser] = true
				}
			}
			if ok := strings.Contains(browser, "MSIE"); ok {
				isMSIE = true
				if _, ok := seenBrowsers[browser]; !ok {
					seenBrowsers[browser] = true
				}
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		email := strings.ReplaceAll(user.Email, "@", " [at] ")
		final := "[" + fmt.Sprint(i) + "] " + user.Name + " <" + email + ">\n"
		io.WriteString(out, final)
	}

	io.WriteString(out, "\nTotal unique browsers "+fmt.Sprint(len(seenBrowsers))+"\n")
}
