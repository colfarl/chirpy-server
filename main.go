package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
)



type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func( w http.ResponseWriter, req *http.Request){
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, req)	
	})
}

func (cfg *apiConfig) middlewareMetricsShow() func(http.ResponseWriter, *http.Request) {
	return func (w http.ResponseWriter, req *http.Request)	{

		numHits := cfg.fileServerHits.Load()
		hitString := fmt.Sprintf("Hits: %d", numHits)
		fmt.Println("Called show; ", hitString)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte(hitString))
	}
}

func (cfg *apiConfig) middlewareMetricsReset() func(http.ResponseWriter, *http.Request) {
	return func (w http.ResponseWriter, req *http.Request)	{
		cfg.fileServerHits.Store(0)
		w.WriteHeader(http.StatusOK)
	}
}


func main() {
	const port = "8080"
	const root = "." 
	
	apiCfg := &apiConfig{
		fileServerHits: atomic.Int32{},
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(root)))
	mux := http.NewServeMux()	
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
		
	mux.HandleFunc("/healthz", func (w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	mux.HandleFunc("/metrics", apiCfg.middlewareMetricsShow()) 
	mux.HandleFunc("/reset", apiCfg.middlewareMetricsReset()) 

	srv := &http.Server{
		Addr: ":" + port,
		Handler: mux,
	}
	
	fmt.Println("Serving on port", port)
	log.Fatal(srv.ListenAndServe())
}
