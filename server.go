package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/urfave/negroni"
)

func main() {
	router := httprouter.New()
	n := negroni.Classic() // Includes some default middlewares
	n.UseHandler(router)

	http.ListenAndServe(":41968", n)
}
