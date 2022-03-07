package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

const ValidAccessToken string = "0123456789"

func ParseXml() []User {
	xmlFile, err := os.Open("./dataset.xml")
	if err != nil {
		panic(err)
	}
	defer xmlFile.Close()
	decoder := xml.NewDecoder(xmlFile)
	users := []User{}
	user := User{}
	firstName, lastName := "", ""
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		if token == nil {
			break
		}
		switch tokenType := token.(type) {
		case xml.StartElement:
			switch tokenType.Name.Local {
			case "id":
				if err = decoder.DecodeElement(&user.Id, &tokenType); err != nil {
					panic(err)
				}
			case "age":
				if err = decoder.DecodeElement(&user.Age, &tokenType); err != nil {
					panic(err)
				}
			case "first_name":
				if err = decoder.DecodeElement(&firstName, &tokenType); err != nil {
					panic(err)
				}
			case "last_name":
				if err = decoder.DecodeElement(&lastName, &tokenType); err != nil {
					panic(err)
				}
			case "about":
				if err = decoder.DecodeElement(&user.About, &tokenType); err != nil {
					panic(err)
				}
			case "gender":
				if err = decoder.DecodeElement(&user.Gender, &tokenType); err != nil {
					panic(err)
				}
			}
		case xml.EndElement:
			switch tokenType.Name.Local {
			case "row":
				user.Name = firstName + " " + lastName
				users = append(users, user)
			}
		}
	}
	return users
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != ValidAccessToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	users := ParseXml()
	length := len(users)

	limit, _ := strconv.Atoi(r.FormValue("limit"))
	offset, _ := strconv.Atoi(r.FormValue("offset"))
	if offset >= length {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	orderField := r.FormValue("order_field")
	orderBy, _ := strconv.Atoi(r.FormValue("order_by"))

	switch orderField {
	case "":
		fallthrough
	case "name":
		switch orderBy {
		case OrderByAsc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name < users[j].Name
			})
		case OrderByDesc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name > users[j].Name
			})
		case OrderByAsIs:
		default:
			errResp := SearchErrorResponse{}
			errResp.Error = "ErrorBadOrderBy"
			errRespJson, err := json.Marshal(errResp)
			if err != nil {
				panic(err)
			}
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, string(errRespJson))
			return
		}
	case "id":
		switch orderBy {
		case OrderByAsc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id < users[j].Id
			})
		case OrderByDesc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id > users[j].Id
			})
		case OrderByAsIs:
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case "age":
		switch orderBy {
		case OrderByAsc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age < users[j].Age
			})
		case OrderByDesc:
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age > users[j].Age
			})
		case OrderByAsIs:
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	default:
		errResp := SearchErrorResponse{}
		errResp.Error = "ErrorBadOrderField"
		errRespJson, err := json.Marshal(errResp)
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, string(errRespJson))
		return
	}

	query := r.FormValue("query")
	border1, border2 := 0, 0

	switch query {
	case "name":
		fallthrough
	case "about":
		border1 = offset
		border2 = offset + limit
		if border2 > length {
			border2 = length
		}
	case "":
		border2 = length
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	usersJson, err := json.Marshal(users[border1:border2])
	if err != nil {
		panic(err)
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(usersJson))
}

func TestFindUsers_NegativeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      -1,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "limit must be > 0"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BigLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      30,
		Offset:     0,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp == nil {
		t.Errorf("Wrong response, expected some non-nil value, got %#v", nil)
	}

	if resp != nil && len(resp.Users) != 25 {
		t.Errorf("Wrong number of users, expected %#v, got %#v", 25, len(resp.Users))
	}

	if err != nil {
		t.Errorf("Wrong error, expected %#v, got %#v", nil, err.Error())
	}
}
func TestFindUsers_NegativeOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     -1,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "offset must be > 0"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_RequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "timeout for limit=6&offset=5&order_by=0&order_field=id&query=name"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BadUrl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         "blablabla",
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "name",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "unknown error Get \"blablabla?limit=6&offset=5&order_by=0&order_field=name&query=name\": unsupported protocol scheme \"\""

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BadAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: "blablabla",
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "Bad AccessToken"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     35,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "SearchServer fatal error"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BadErrorJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    2,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "cant unpack error json: unexpected end of JSON input"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BadOrderField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "blablabla",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "OrderFeld blablabla invalid"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_UnknownBadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "name",
		OrderBy:    2,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "unknown bad request error: ErrorBadOrderBy"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_BadResultJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp != nil {
		t.Errorf("Wrong response, expected %#v, got %#v", nil, resp)
	}

	expErr := "cant unpack result json: unexpected end of JSON input"

	if err.Error() != expErr {
		t.Errorf("Wrong error, expected %#v, got %#v", expErr, err.Error())
	}
}
func TestFindUsers_ResponseWithNextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     5,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp == nil {
		t.Errorf("Wrong response, expected some non-nil value, got %#v", nil)
	}

	if resp != nil && resp.NextPage != true {
		t.Errorf("Wrong number of users, expected %#v, got %#v", true, resp.NextPage)
	}

	if err != nil {
		t.Errorf("Wrong error, expected %#v, got %#v", nil, err.Error())
	}
}
func TestFindUsers_ResponseWithoutNextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := &SearchClient{
		AccessToken: ValidAccessToken,
		URL:         server.URL,
	}

	request := SearchRequest{
		Limit:      5,
		Offset:     33,
		Query:      "name",
		OrderField: "id",
		OrderBy:    OrderByAsIs,
	}

	resp, err := client.FindUsers(request)

	if resp == nil {
		t.Errorf("Wrong response, expected some non-nil value, got %#v", nil)
	}

	if resp != nil && resp.NextPage != false {
		t.Errorf("Wrong number of users, expected %#v, got %#v", false, resp.NextPage)
	}

	if err != nil {
		t.Errorf("Wrong error, expected %#v, got %#v", nil, err.Error())
	}
}
