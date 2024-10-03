# zqlsession
A custom session store for [scs](https://github.com/alexedwards/scs) using the
[zombiezen sqlite interface](https://github.com/zombiezen/go-sqlite).

# usage
```go
func main() {
	// Open your database.
	db, err := sqlitex.NewPool("app.db", nil)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	// Setup session storage.
	sessionManager := scs.New()
	sessionManager.Store = zqlsession.New(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/put", putHandler)
	mux.HandleFunc("/get", getHandler)

	// Wrap your handlers with the LoadAndSave() middleware.
	http.ListenAndServe(":4000", sessionManager.LoadAndSave(mux))
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	// Store a new key and value in the session data.
	sessionManager.Put(r.Context(), "message", "Hello from a session!")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	// Use the GetString helper to retrieve the string value associated with a
	// key. The zero value is returned if the key does not exist.
	msg := sessionManager.GetString(r.Context(), "message")
	io.WriteString(w, msg)
}
```

# author
Written and maintained by Dakota Walsh.
Up-to-date sources can be found at https://git.sr.ht/~kota/zqlsession/

# license
GNU LGPL version 3 or later, see LICENSE.
