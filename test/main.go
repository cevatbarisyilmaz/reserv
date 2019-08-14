package main

import (
	"github.com/cevatbarisyilmaz/reserv"
	"log"
	"net/http"
)

type apiHandler struct {

}

func (handler *apiHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, err := writer.Write([]byte(request.URL.String()))
	if err != nil {
		log.Println(err)
	}
}

func main() {
	res, err := reserv.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	res.APIHandler = &apiHandler{}
	err = res.Run()
	if err != nil {
		log.Fatal(err)
	}
}
