package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func PackError(text string) []byte {
	data, err := json.Marshal(map[string]interface{}{
		"error": text,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func PackResponse(response interface{}) []byte {
	data, err := json.Marshal(map[string]interface{}{
		"error":    "",
		"response": response,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func (api *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		api.wrapperProfile(w, r)
	case "/user/create":
		api.wrapperCreate(w, r)
	default:
		data := PackError("unknown method")
		w.WriteHeader(http.StatusNotFound)
		w.Write(data)
	}
}

func (api *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		api.wrapperCreate(w, r)
	default:
		data := PackError("unknown method")
		w.WriteHeader(http.StatusNotFound)
		w.Write(data)
	}
}

func (api *MyApi) wrapperProfile(w http.ResponseWriter, r *http.Request) {
	params:=ProfileParams{}
	params.Login = r.FormValue("login")
	if params.Login == "" {
		data := PackError("login must me not empty")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	ctx := r.Context()
	resp, err := api.Profile(ctx, params)
	if err != nil {
		if apiErr, ok := err.(ApiError); ok {
			data := PackError(apiErr.Error())
			w.WriteHeader(apiErr.HTTPStatus)
			w.Write(data)
			return
		} else {
			data := PackError(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	data := PackResponse(resp)
	w.Write(data)
}

func (api *MyApi) wrapperCreate(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		data := PackError("unauthorized")
		w.Write(data)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		data := PackError("bad method")
		w.Write(data)
		return
	}
	params:=CreateParams{}
	params.Login = r.FormValue("login")
	if params.Login == "" {
		data := PackError("login must me not empty")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	if len(params.Login) < 10 {
		data := PackError("login len must be >= 10")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	params.Name = r.FormValue("full_name")
	params.Status = r.FormValue("status")
	if params.Status == "" {
		params.Status = "user"
	}
	if params.Status != "user" && params.Status != "moderator" && params.Status != "admin" {
		data := PackError("status must be one of [user, moderator, admin]")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	age, err:=strconv.Atoi(r.FormValue("age"))
	if err != nil {
		data := PackError("age must be int")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	params.Age = age
	if params.Age < 0 {
		data := PackError("age must be >= 0")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	if params.Age > 128 {
		data := PackError("age must be <= 128")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	ctx := r.Context()
	resp, err := api.Create(ctx, params)
	if err != nil {
		if apiErr, ok := err.(ApiError); ok {
			data := PackError(apiErr.Error())
			w.WriteHeader(apiErr.HTTPStatus)
			w.Write(data)
			return
		} else {
			data := PackError(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	data := PackResponse(resp)
	w.Write(data)
}

func (api *OtherApi) wrapperCreate(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		data := PackError("unauthorized")
		w.Write(data)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		data := PackError("bad method")
		w.Write(data)
		return
	}
	params:=OtherCreateParams{}
	params.Username = r.FormValue("username")
	if params.Username == "" {
		data := PackError("username must me not empty")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	if len(params.Username) < 3 {
		data := PackError("username len must be >= 3")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	params.Name = r.FormValue("account_name")
	params.Class = r.FormValue("class")
	if params.Class == "" {
		params.Class = "warrior"
	}
	if params.Class != "warrior" && params.Class != "sorcerer" && params.Class != "rouge" {
		data := PackError("class must be one of [warrior, sorcerer, rouge]")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	level, err:=strconv.Atoi(r.FormValue("level"))
	if err != nil {
		data := PackError("level must be int")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	params.Level = level
	if params.Level < 1 {
		data := PackError("level must be >= 1")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	if params.Level > 50 {
		data := PackError("level must be <= 50")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	ctx := r.Context()
	resp, err := api.Create(ctx, params)
	if err != nil {
		if apiErr, ok := err.(ApiError); ok {
			data := PackError(apiErr.Error())
			w.WriteHeader(apiErr.HTTPStatus)
			w.Write(data)
			return
		} else {
			data := PackError(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	data := PackResponse(resp)
	w.Write(data)
}

