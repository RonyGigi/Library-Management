package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/url"

	"github.com/codegangsta/negroni"

	gmux "github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/yosssi/ace"
	"gopkg.in/gorp.v1"
)

type Book struct {
	PK             int64  `db:"pk"`
	Title          string `db:"title"`
	Author         string `db:"author"`
	Classification string `db:"classification"`
	ID             string `db:"id"`
}

type Page struct {
	Books []Book
}

type SearchResult struct {
	Title  string `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year   string `xml:"hyr,attr"`
	ID     string `xml:"owi,attr"`
}

var db *sql.DB
var dbmap *gorp.DbMap

func initDB() {
	db, _ = sql.Open("sqlite3", "dev.db")
	dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	dbmap.AddTableWithName(Book{}, "book").SetKeys(true, "pk")
	dbmap.CreateTablesIfNotExists()
}

func verifyDatabase(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if err := db.Ping(); err != nil {
		// fmt.Println("Errrrr'rrr")
		// return
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, r)
}
func main() {
	initDB()
	// template,err := template.Must(template.ParseFiles("templates/index.html"))
	db, _ = sql.Open("sqlite3", "dev.db")
	mux := gmux.NewRouter()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template, err := ace.Load("templates/index", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		p := Page{Books: []Book{}}

		if _, err := dbmap.Select(&p.Books, "select * from book"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// rows, _ := db.Query("select pk,title,author,classification from book")
		// for rows.Next() {
		// 	var b Book
		// 	rows.Scan(&b.PK, &b.Title, &b.Author, &b.Classification)
		// 	p.Books = append(p.Books, b)
		// }

		// if err := templates.ExecuteTemplate(w, "index.html", p); err != nil {
		if err := template.Execute(w, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// db.Close()

	}).Methods("GET")
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []SearchResult
		var err error
		if results, err = search(r.FormValue("search")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		encoder := json.NewEncoder(w)
		// fmt.Println(results)
		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	}).Methods("POST")
	mux.HandleFunc("/books/{pk}", func(w http.ResponseWriter, r *http.Request) {
		// if _, err := db.Exec("delete from book where PK=?", gmux.Vars(r)["pk"]); err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// 	return
		// }
		pk, _ := strconv.ParseInt(gmux.Vars(r)["pk"], 10, 64)
		if _, err := dbmap.Delete(&Book{pk, "", "", "", ""}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("DELETE")
	mux.HandleFunc("/books/add", func(w http.ResponseWriter, r *http.Request) {
		var book ClassifyBookResponse
		var err error
		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// if err = db.Ping(); err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// }

		// result, err := db.Exec("insert into book (pk,title,author,id,classification) values (?,?,?,?,?)", nil, book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// }
		// pk, _ := result.LastInsertId()

		b := Book{
			PK:             -1,
			Title:          book.BookData.Title,
			Author:         book.BookData.Author,
			Classification: book.Classification.MostPopular,
			ID:             book.BookData.ID,
		}
		if err = dbmap.Insert(&b); err != nil {
			fmt.Println("Errorrrrr")
			return
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("PUT")
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(verifyDatabase))
	n.UseHandler(mux)
	n.Run(":8080")
	// fmt.Println(http.ListenAndServe(":8080", nil))
}

type ClassifySearchResponse struct {
	Results []SearchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title  string `xml:"title,attr"`
		Author string `xml:"author,attr"`
		Year   string `xml:"hyr,attr"`
		ID     string `xml:"owi,attr"`
	} `xml:"work"`
	Classification struct {
		MostPopular string `xml:"sfa,attr"`
	} `xml:"recommendations>ddc>mostPopular"`
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&owi=" + url.QueryEscape(id))
	if err != nil {
		return ClassifyBookResponse{}, err
	}
	err = xml.Unmarshal(body, &c)
	return c, err
}

func search(query string) ([]SearchResult, error) {
	var c ClassifySearchResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&title=" + url.QueryEscape(query))
	if err != nil {
		return []SearchResult{}, err
	}

	err = xml.Unmarshal(body, &c)

	// fmt.Println(string(body))
	return c.Results, err
}

func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
