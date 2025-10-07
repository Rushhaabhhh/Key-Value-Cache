package main

import (
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	maxKeyValueSize = 256
	numShards       = 4
)

func djb2Hash(s string) uint64 {
	var hash uint64 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint64(s[i])
	}
	return hash
}


type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type GetResponse struct {
	Status string `json:"status"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

var (
	putSuccessBytes  = []byte(`{"status":"OK","message":"Key inserted/updated successfully."}`)
	keyNotFoundBytes = []byte(`{"status":"ERROR","message":"Key not found."}`)
)


func requestHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Method()) {
	case "POST":
		if string(ctx.Path()) == "/put" {
			body := ctx.PostBody()
			var req PutRequest
			if err := json.Unmarshal(body, &req); err != nil {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			if req.Key == "" {
				ctx.Error("Key parameter is required", fasthttp.StatusBadRequest)
				return
			}
			if len(req.Key) > maxKeyValueSize || len(req.Value) > maxKeyValueSize {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			ctx.Response.Header.Set("Content-Type", "application/json")
			ctx.SetBody(putSuccessBytes)
			return
		}
	case "GET":
		if string(ctx.Path()) == "/get" {
			key := string(ctx.QueryArgs().Peek("key"))
			if key == "" {
				ctx.Error("Key parameter is required", fasthttp.StatusBadRequest)
				return
			}

			resp := GetResponse{
				Status: "OK",
				Key:    key,
				Value:  value,
			}
			respJSON, err := json.Marshal(resp)
			if err != nil {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			ctx.Response.Header.Set("Content-Type", "application/json")
			ctx.SetBody(respJSON)
			return
		}
	}
	ctx.Error("Unsupported request", fasthttp.StatusNotFound)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	s := &fasthttp.Server{
		Handler:              requestHandler,
		Name:                 "KeyValueCache",
		ReadTimeout:          5 * time.Second,
		WriteTimeout:         5 * time.Second,
		MaxConnsPerIP:        0,
		MaxKeepaliveDuration: 60 * time.Second,
	}
	log.Printf("Starting fasthttp server on %s", ":7171")
	if err := s.ListenAndServe(":7171"); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}