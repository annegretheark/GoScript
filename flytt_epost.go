package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Regel struct {
	Kategori string
	Flyttes  string
}

func main() {
	server := os.Getenv("MAIL_SERVER")
	user := os.Getenv("MAIL_USER")
	pass := os.Getenv("MAIL_PASSWORD")

	if server == "" || user == "" || pass == "" {
		log.Fatal("Mangler MAIL_SERVER, MAIL_USER eller MAIL_PASSWORD")
	}

	regler, err := lesRegler("avsendere.csv")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Kobler til e-post...")

	c, err := client.DialTLS(server+":993", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Logout()

	if err := c.Login(user, pass); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Innlogget")

	_, err = c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(1, 0)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{
		section.FetchItem(),
	}

	messages := make(chan *imap.Message, 50)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	flyttet := 0

	for msg := range messages {
		r := msg.GetBody(section)
		if r == nil {
			continue
		}

		m, err := mail.ReadMessage(r)
		if err != nil {
			continue
		}

		from := m.Header.Get("From")
		if from == "" {
			continue
		}

		addr, err := mail.ParseAddress(from)
		if err != nil {
			continue
		}

		epost := strings.ToLower(strings.TrimSpace(addr.Address))

		regel, finnes := regler[epost]
		if !finnes {
			continue
		}

		if strings.ToLower(strings.TrimSpace(regel.Flyttes)) != "ja" {
			continue
		}

		mappe := strings.TrimSpace(regel.Kategori)
		if mappe == "" {
			mappe = "MAS"
		}

		seq := new(imap.SeqSet)
		seq.AddNum(msg.SeqNum)

		err = c.Copy(seq, mappe)
		if err != nil {
			fmt.Println("Kunne ikke kopiere til mappe:", mappe, "-", err)
			continue
		}

		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}

		err = c.Store(seq, item, flags, nil)
		if err != nil {
			fmt.Println("Kunne ikke merke som slettet:", err)
			continue
		}

		fmt.Println("Flyttet:", epost, "til", mappe)
		flyttet++
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	if flyttet > 0 {
		err = c.Expunge(nil)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("Ferdig. Flyttet %d e-poster.\n", flyttet)
}

func lesRegler(filnavn string) (map[string]Regel, error) {
	file, err := os.Open(filnavn)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	regler := make(map[string]Regel)

	for i, row := range rows {
		if i == 0 {
			continue
		}

		if len(row) < 4 {
			continue
		}

		epost := strings.ToLower(strings.TrimSpace(row[0]))
		kategori := strings.TrimSpace(row[2])
		flyttes := strings.TrimSpace(row[3])

		if epost == "" {
			continue
		}

		regler[epost] = Regel{
			Kategori: kategori,
			Flyttes:  flyttes,
		}
	}

	return regler, nil
}
