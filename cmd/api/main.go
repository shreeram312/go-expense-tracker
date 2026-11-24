package main

import (
	"expense-tracker/internal/httpapi"
	"fmt"
)

func main(){
	fmt.Println("Starting Expense tracker")
	greeting:= httpapi.Greeting()
	fmt.Println(greeting)
}