package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

const header string = `package main

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
}`

type MethodJson struct {
	Url    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type MethodInfo struct {
	Name          string
	ParamTypeName string
	ApiTypeName   string
	Json          MethodJson
}

type Apivalidator struct {
	Required  bool
	ParamName string
	Enum      []string
	Default   string
	Min       string
	Max       string
}

type Param struct {
	Name         string
	Type         string
	Apivalidator Apivalidator
}

type ParamInfo struct {
	TypeName string
	Params   []Param
}

func main() {
	file, _ := os.ReadFile(os.Args[1])
	fileSet := token.NewFileSet()
	node, _ := parser.ParseFile(fileSet, os.Args[1], file, parser.ParseComments)
	offset := node.Pos()

	uniApiTypeNames := []string{}
	methodInfos := []MethodInfo{}
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Doc == nil {
			continue
		}
		needCodegen := false
		for _, comment := range funcDecl.Doc.List {
			needCodegen = needCodegen || strings.HasPrefix(comment.Text, "// apigen:api")
		}
		if !needCodegen {
			continue
		}

		methodInfo := MethodInfo{}
		methodInfo.Name = funcDecl.Name.Name
		methodInfo.ParamTypeName = string(file[funcDecl.Type.Params.List[1].Type.Pos()-offset : funcDecl.Type.Params.List[1].Type.End()-offset])
		methodInfo.ApiTypeName = string(file[funcDecl.Recv.List[0].Type.Pos()-offset : funcDecl.Recv.List[0].Type.End()-offset])
		_ = json.Unmarshal([]byte(strings.Replace(funcDecl.Doc.List[0].Text, "// apigen:api ", "", 1)), &methodInfo.Json)
		methodInfos = append(methodInfos, methodInfo)

		notSeenBefore := true
		for _, apiTypeName := range uniApiTypeNames {
			if apiTypeName == methodInfo.ApiTypeName {
				notSeenBefore = false
			}
		}
		if notSeenBefore {
			uniApiTypeNames = append(uniApiTypeNames, methodInfo.ApiTypeName)
		}
	}

	paramInfos := map[string]ParamInfo{}
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				continue
			}

			paramType := false
			for _, methodInfo := range methodInfos {
				if currType.Name.Name == methodInfo.ParamTypeName {
					paramType = true
				}
			}
			if !paramType {
				continue
			}

			paramInfo := ParamInfo{}
			paramInfo.TypeName = currType.Name.Name
			for _, field := range currStruct.Fields.List {
				param := Param{}
				param.Name = field.Names[0].Name
				param.Type = field.Type.(*ast.Ident).Name
				if field.Tag != nil {
					apivalidator := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("apivalidator")
					pairs := strings.Split(apivalidator, ",")
					setParamName := false
					for _, pair := range pairs {
						splittedPair := strings.Split(pair, "=")
						key, value := "", ""
						if len(splittedPair) == 1 {
							key = splittedPair[0]
						} else {
							key = splittedPair[0]
							value = splittedPair[1]
						}
						switch key {
						case "required":
							param.Apivalidator.Required = true
						case "paramname":
							setParamName = true
							param.Apivalidator.ParamName = value
						case "enum":
							param.Apivalidator.Enum = strings.Split(value, "|")
						case "default":
							param.Apivalidator.Default = value
						case "min":
							param.Apivalidator.Min = value
						case "max":
							param.Apivalidator.Max = value
						}
						if !setParamName {
							param.Apivalidator.ParamName = strings.ToLower(param.Name)
						}
					}
				}
				paramInfo.Params = append(paramInfo.Params, param)
			}
			paramInfos[currType.Name.Name] = paramInfo
		}
	}

	out, _ := os.Create(os.Args[2])
	fmt.Fprintln(out, header)
	fmt.Fprintln(out)

	for _, apiTypeName := range uniApiTypeNames {
		fmt.Fprintln(out, "func (api "+apiTypeName+") ServeHTTP(w http.ResponseWriter, r *http.Request) {")
		fmt.Fprintln(out, "\tswitch r.URL.Path {")

		for _, methodInfo := range methodInfos {
			if methodInfo.ApiTypeName == apiTypeName {
				fmt.Fprintln(out, "\tcase \""+methodInfo.Json.Url+"\":")
				fmt.Fprintln(out, "\t\tapi.wrapper"+methodInfo.Name+"(w, r)")
			}
		}

		fmt.Fprintln(out, "\tdefault:")
		fmt.Fprintln(out, "\t\tdata := PackError(\"unknown method\")")
		fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusNotFound)")
		fmt.Fprintln(out, "\t\tw.Write(data)")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintln(out, "}")
		fmt.Fprintln(out)
	}

	for _, methodInfo := range methodInfos {
		fmt.Fprintln(out, "func (api "+methodInfo.ApiTypeName+") wrapper"+methodInfo.Name+"(w http.ResponseWriter, r *http.Request) {")

		if methodInfo.Json.Auth {
			fmt.Fprintln(out, "\tif r.Header.Get(\"X-Auth\") != \"100500\" {")
			fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusForbidden)")
			fmt.Fprintln(out, "\t\tdata := PackError(\"unauthorized\")")
			fmt.Fprintln(out, "\t\tw.Write(data)")
			fmt.Fprintln(out, "\t\treturn")
			fmt.Fprintln(out, "\t}")
		}

		if methodInfo.Json.Method != "" {
			fmt.Fprintln(out, "\tif r.Method != \""+methodInfo.Json.Method+"\" {")
			fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusNotAcceptable)")
			fmt.Fprintln(out, "\t\tdata := PackError(\"bad method\")")
			fmt.Fprintln(out, "\t\tw.Write(data)")
			fmt.Fprintln(out, "\t\treturn")
			fmt.Fprintln(out, "\t}")
		}

		fmt.Fprintln(out, "\tparams:="+methodInfo.ParamTypeName+"{}")
		for _, param := range paramInfos[methodInfo.ParamTypeName].Params {
			if param.Type == "string" {
				fmt.Fprintln(out, "\tparams."+param.Name+" = r.FormValue(\""+param.Apivalidator.ParamName+"\")")
			} else {
				fmt.Fprintln(out, "\t"+param.Apivalidator.ParamName+", err:=strconv.Atoi(r.FormValue(\""+param.Apivalidator.ParamName+"\"))")
				fmt.Fprintln(out, "\tif err != nil {")
				fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must be int\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\tw.Write(data)")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
				fmt.Fprintln(out, "\tparams."+param.Name+" = "+param.Apivalidator.ParamName)
			}

			if param.Apivalidator.Required {
				if param.Type == "string" {
					fmt.Fprintln(out, "\tif params."+param.Name+" == \"\" {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must me not empty\")")
				} else {
					fmt.Fprintln(out, "\tif params."+param.Name+" == 0 {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must me not default\")")
				}
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\tw.Write(data)")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if param.Apivalidator.Default != "" {
				if param.Type == "string" {
					fmt.Fprintln(out, "\tif params."+param.Name+" == \"\" {")
					fmt.Fprintln(out, "\t\tparams."+param.Name+" = \""+param.Apivalidator.Default+"\"")
				} else {
					fmt.Fprintln(out, "\tif params."+param.Name+" == 0 {")
					fmt.Fprintln(out, "\t\tparams."+param.Name+" = "+param.Apivalidator.Default)
				}
				fmt.Fprintln(out, "\t}")
			}

			if param.Apivalidator.Enum != nil {
				fmt.Fprint(out, "\tif ")
				for i, enum := range param.Apivalidator.Enum {
					if i == 0 {
						if param.Type == "string" {
							fmt.Fprint(out, "params."+param.Name+" != \""+enum+"\"")
						} else {
							fmt.Fprint(out, "params."+param.Name+" != "+enum)
						}
					} else {
						if param.Type == "string" {
							fmt.Fprint(out, " && params."+param.Name+" != \""+enum+"\"")
						} else {
							fmt.Fprint(out, " && params."+param.Name+" != "+enum)
						}
					}
				}
				fmt.Fprintln(out, " {")
				fmt.Fprint(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must be one of [")
				for i, enum := range param.Apivalidator.Enum {
					if i == 0 {
						fmt.Fprint(out, enum)
					} else {
						fmt.Fprint(out, ", "+enum)
					}
				}
				fmt.Fprintln(out, "]\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\tw.Write(data)")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if param.Apivalidator.Min != "" {
				if param.Type == "string" {
					fmt.Fprintln(out, "\tif len(params."+param.Name+") < "+param.Apivalidator.Min+" {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" len must be >= "+param.Apivalidator.Min+"\")")
				} else {
					fmt.Fprintln(out, "\tif params."+param.Name+" < "+param.Apivalidator.Min+" {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must be >= "+param.Apivalidator.Min+"\")")
				}
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\tw.Write(data)")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if param.Apivalidator.Max != "" {
				if param.Type == "string" {
					fmt.Fprintln(out, "\tif len(params."+param.Name+") > "+param.Apivalidator.Max+" {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" len must be <= "+param.Apivalidator.Max+"\")")
				} else {
					fmt.Fprintln(out, "\tif params."+param.Name+" > "+param.Apivalidator.Max+" {")
					fmt.Fprintln(out, "\t\tdata := PackError(\""+param.Apivalidator.ParamName+" must be <= "+param.Apivalidator.Max+"\")")
				}
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\tw.Write(data)")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}
		}

		fmt.Fprintln(out, "\tctx := r.Context()")
		fmt.Fprintln(out, "\tresp, err := api."+methodInfo.Name+"(ctx, params)")
		fmt.Fprintln(out, "\tif err != nil {")
		fmt.Fprintln(out, "\t\tif apiErr, ok := err.(ApiError); ok {")
		fmt.Fprintln(out, "\t\t\tdata := PackError(apiErr.Error())")
		fmt.Fprintln(out, "\t\t\tw.WriteHeader(apiErr.HTTPStatus)")
		fmt.Fprintln(out, "\t\t\tw.Write(data)")
		fmt.Fprintln(out, "\t\t\treturn")
		fmt.Fprintln(out, "\t\t} else {")
		fmt.Fprintln(out, "\t\t\tdata := PackError(err.Error())")
		fmt.Fprintln(out, "\t\t\tw.WriteHeader(http.StatusInternalServerError)")
		fmt.Fprintln(out, "\t\t\tw.Write(data)")
		fmt.Fprintln(out, "\t\t\treturn")
		fmt.Fprintln(out, "\t\t}")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintln(out, "\tw.WriteHeader(http.StatusOK)")
		fmt.Fprintln(out, "\tdata := PackResponse(resp)")
		fmt.Fprintln(out, "\tw.Write(data)")
		fmt.Fprintln(out, "}")
		fmt.Fprintln(out)
	}
}
