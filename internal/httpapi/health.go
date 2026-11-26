package httpapi

import "net/http"



func handleHealth(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}


func Routes() http.Handler{
	mux:=http.NewServeMux()
	mux.HandleFunc("GET /health",handleHealth)
	return mux
}