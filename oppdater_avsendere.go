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

const csvFil = "avsendere.csv"

func main() {
	server := os.Getenv("MAIL_SERVER")
	user := os.Getenv("MAIL_USER")
	pass := os.Getenv("MAIL_PASSWORD")

	if server == "" || user == "" || pass == "" {
		log.Fatal("Mangler MAIL_SERVER, MAIL_USER eller MAIL_PASSWORD")
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
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 50)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	nyeAvsendere := make(map[string]string)

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
		navn := strings.TrimSpace(addr.Name)

		if navn == "" {
			navn = epost
		}

		nyeAvsendere[epost] = navn
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	err = oppdaterAvsendereCSV(csvFil, nyeAvsendere)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Ferdig. avsendere.csv er oppdatert.")
}

func oppdaterAvsendereCSV(filnavn string, avsendere map[string]string) error {
	eksisterende := make(map[string]bool)

	if file, err := os.Open(filnavn); err == nil {
		defer file.Close()

		reader := csv.NewReader(file)
		reader.Comma = ';'
		reader.FieldsPerRecord = -1

		rows, _ := reader.ReadAll()
		for _, row := range rows {
			if len(row) > 0 {
				epost := strings.ToLower(strings.TrimSpace(row[0]))
				eksisterende[epost] = true
			}
		}
	}

	file, err := os.OpenFile(filnavn, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// Hvis filen er ny/tom, legg inn header
	info, _ := file.Stat()
	if info.Size() == 0 {
		writer.Write([]string{"epost", "navn", "kategori", "flyttes"})
	}

	lagtTil := 0

	for epost, navn := range avsendere {
		if epost == "" {
			continue
		}

		if !eksisterende[epost] {
			err := writer.Write([]string{epost, navn, "", "nei"})
			if err != nil {
				return err
			}

			fmt.Println("La til ny avsender:", epost, "-", navn)
			eksisterende[epost] = true
			lagtTil++
		}
	}

	fmt.Printf("La til %d nye avsendere\n", lagtTil)

	return nil
}
