package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("gmail-go-quickstart.json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved credentials
	// at ~/.credentials/gmail-go-quickstart.json
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config)

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
	}

	user := "me"
	listMessages(srv, user)
}

func listMessages(srv *gmail.Service, user string) {
	msgs, err := srv.Users.Messages.List(user).Do()

	if err != nil {
		log.Fatalf("Unable to retrieve message ids. %v", err)
	}

	if len(msgs.Messages) > 0 {
		fmt.Print("Messages:\n")
		for _, m := range msgs.Messages {
			fmt.Printf("- %s\n", m.Id)

		}
	} else {
		fmt.Print("No messages found.")
		return
	}

	fmt.Println("Enter message id: ")
	var msgId string
	if _, err := fmt.Scan(&msgId); err != nil {
		log.Fatalf("Unable to read msg id %v", err)
	}

	getMessage(srv, user, msgId)

}

func getMessage(srv *gmail.Service, user string, msgId string) {

	msg, err := srv.Users.Messages.Get(user, msgId).Do()

	if err != nil {
		log.Fatalf("Unable to retrieve message. %v", err)
		return
	}

	fmt.Println("Message Metadata and Headers:")
	fmt.Println("*********************************************")
	fmt.Println("Message Id:", msg.Id)
	fmt.Println("Thread Id:", msg.ThreadId)
	fmt.Println("History Id:", msg.HistoryId)

	fmt.Println("Internal Date:", msg.InternalDate)
	fmt.Println("Size Estimate:", msg.SizeEstimate)

	fmt.Println()
	fmt.Println()

	for _, header := range msg.Payload.Headers {
		fmt.Println(header.Name, ":", header.Value)
	}

	fmt.Println()
	fmt.Println()

	listLabels(msg.LabelIds)
	fmt.Println("*********************************************")

	fmt.Println()
	fmt.Println()

	fmt.Println("Message snippet:")
	fmt.Println(msg.Snippet)
	fmt.Println()
	fmt.Println()

	fmt.Println("Body of message")
	for _, part := range msg.Payload.Parts {

		if part.MimeType == "text/html" {
			data, _ := base64.RawURLEncoding.DecodeString(part.Body.Data)
			html := string(data)
			fmt.Println(html)
		}
	}
	fmt.Println()
	fmt.Println()
	fmt.Println("Attachments:")
	for _, part := range msg.Payload.Parts {

		if part.MimeType == "application/octet-stream" {
			fmt.Println("Filename: ", part.Filename)
			fmt.Println("Id: ", part.Body.AttachmentId)
			fmt.Println("Attachment size: ", part.Body.Size)
			err := saveAttachment(srv, user, msg.Id, part.Body.AttachmentId, part.Filename)
			if err != nil {
				log.Fatal("Could not save attachment.")
			} else {
				fmt.Println("Attachment downloaded")
			}

		}
	}

	fmt.Println("*********************************************")

}

func listLabels(labels []string) {

	if len(labels) > 0 {
		fmt.Print("Labels:\n")
		for _, l := range labels {
			fmt.Printf("- %s\n", l)
		}
	} else {
		fmt.Print("No labels found.")
	}
}

func saveAttachment(srv *gmail.Service, user, msgId, attachId, filename string) error {
	attach, _ := srv.Users.Messages.Attachments.Get(user, msgId, attachId).Do()
	decoded, err := base64.URLEncoding.DecodeString(attach.Data)

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to create attachment file: %v", err)
	}
	defer f.Close()

	_, err = f.Write(decoded)
	defer f.Close()

	return err

}
