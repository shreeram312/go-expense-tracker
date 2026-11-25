package main

import (
	"expense-tracker/internal/httpapi"
	"expense-tracker/internal/store"
	"fmt"
)

func main() {
	fmt.Println("Starting Expense tracker")
	greeting := httpapi.Greeting()
	fmt.Println(greeting)


	st:=store.NewMemoryStore()
	st.Create(store.Expense{
		AmountPaise: 23230,
		Note: "Lunch",
		SpentOn: "2026-11-24",
	})

	st.Create(store.Expense{AmountPaise: 15000, Note: "taxi", SpentOn: "2026-11-24"})

	fmt.Println(st.List())

}
	