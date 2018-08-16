package main

import (
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	bolt "github.com/coreos/bbolt"
)

const (
	comiketID   = "398275988116602902"
	reitaisaiID = "398389744721199104"
	m3ID        = "398390029849722881"
)

var (
	db        *bolt.DB
	templates = template.Must(template.ParseGlob("tmpl/*.html"))
)

// util
// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// reverse returns a slice of messages reversed (e.g [a, b, c] => [c, b, a])
func reverse(a []*discordgo.Message) []*discordgo.Message {
	var len = len(a)
	var halfLen = len / 2
	for i := 0; i < halfLen; i++ {
		var temp = a[i]
		a[i] = a[len-1-i]
		a[len-1-i] = temp
	}
	return a
}

// database
func messages(db *bolt.DB, bucket string) ([]string, error) {
	var s []string
	err := db.View(func(tx *bolt.Tx) error {
		messages := tx.Bucket([]byte(bucket))
		messages.ForEach(func(k, v []byte) error {
			decoded, err := base64.StdEncoding.DecodeString(string(v))
			if err != nil {
				fmt.Println("decode error:", err)
				return err
			}
			s = append(s, string(decoded))
			return nil
		})
		if err != nil {
			fmt.Println("error iterating over messages:", err)
			return err
		}
	})
	if err != nil {
		fmt.Printf("viewing bucket failed: %s\n:", err)
		return nil, err
	}
	return s, nil
}

// http
func render(w http.ResponseWriter, tmpl string, d interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html>this site scrapes the doujinstyle discord<br><a href='/comiket'>comiket</a>, <a href='/reitaisai'>reitaisai</a>, <a href='/m3'>m3</a><br>this server doesn't host SHIT")
	return
}

func handler(db *bolt.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b string
		switch html.EscapeString(r.URL.Path[1:]) {
		case "comiket":
			b = "messages"
		case "m3":
			b = "m3"
		case "reitaisai":
			b = "reitaisai"

		default:
			http.Error(w, "not a valid URL idiot", http.StatusNotFound)

		}
		data, err := messages(db, b)
		if err != nil {
			log.Printf("viewing %s: %s\n", b, err)
		}
		render(w, "messages", data)
	})
}

func main() {
	var tk = flag.String("token", "cock", "Discord API token (user)")
	flag.Parse()
	if *tk == "cock" {
		fmt.Println("=> you didn't enter a token. use ./disc94 -token='token'")
		return
	}
	fmt.Println("=> creating new session")
	discord, err := discordgo.New(*tk)
	if err != nil {
		log.Panicln("discordgo:", err)
		return
	}

	fmt.Println("=> opening connection on session")
	err = discord.Open()
	if err != nil {
		log.Panicln("opening connection:", err)
	}
	fmt.Println("=> DATABASEDATABASE")
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=> adding handlers")
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		var bucket string
		if m.Author.ID == s.State.User.ID {
			return
		}
		switch m.ChannelID {
		case comiketID:
			bucket = "messages"
		case reitaisaiID:
			bucket = "reitaisai"
		case m3ID:
			bucket = "m3"
		default:
			return
		}
		b64 := base64.StdEncoding.EncodeToString([]byte(m.Content))
		err := db.Update(func(tx *bolt.Tx) error {
			messages, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				log.Printf("create bucket: %s", err)
				return err
			}
			id, err := messages.NextSequence()
			if err != nil {
				log.Printf("finding next sequence: %s", err)
				return err
			}
			err = messages.Put(itob(id), []byte(b64))
			return err
		})
		if err != nil {
			fmt.Printf("=> failed to put message in DB: %s", err)
		}
		fmt.Println(m.Content)
		return
	})

	fmt.Println("=> setting up http")

	http.HandleFunc("/", home)
	http.Handle("/comiket", handler(db))
	http.Handle("/m3", handler(db))
	http.Handle("/reitaisai", handler(db))
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("img"))))

	fmt.Println("scraping bot is now running.  press CTRL-C to exit.")

	log.Fatal(http.ListenAndServe(":1337", nil))
	// Close connections
	discord.Close()

	return
}
