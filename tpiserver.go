package main

import (
    "fmt"
	"net/http"
    "log"
    "encoding/json"
    "flag"
    "time"
    "io/ioutil"
    "sync"
    "errors"
    "os"
    "bytes"

    "github.com/dgrijalva/jwt-go"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/service/s3"
)

type Update struct {
    Current interface{} `json:"value"`
    Min float64 `json:"min"`
    Max float64 `json:"max"`
}

type Store struct {
    mux sync.Mutex
    values map[string]*Update
}

type TokenClaims struct {
    UserId string `json:"user_id"`
    Role string `json:"role"`
    PubsubPerms map[string][]string `json:"pubsub_perms"`
    jwt.StandardClaims
}

const (
    updatePeriod = time.Second * 5
    S3_REGION = "us-east-1"
    S3_BUCKET = "plays-itself-filters"
    S3_KEY = "filters.json"
    tokenPrefix = "Bearer "
    tokenDuration = 30
    twitchID = "207678528"
    twitchRole = "external"
)

var (
    port = flag.String("port", "8001", "port to serve app on")
    store = Store{values: make(map[string]*Update)}
    tokenSecret = flag.String("secret", "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk", "JWT Secret")
)


func checkToken(authString string) error {
    if len(authString) < len(tokenPrefix) {
        return errors.New("Invalid or missing Authorization header")
    }
    tokenString := authString[len(tokenPrefix):]

    _, err := jwt.ParseWithClaims(tokenString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(*tokenSecret), nil
    })

    return err
}

func serveIngest(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        msg := fmt.Sprintf("Method %v is not allowed", r.Method)
        http.Error(w, msg, http.StatusMethodNotAllowed)
        return
    }

    if r.Body == nil {
        http.Error(w, "No data was sent", http.StatusBadRequest)
        return
    }

    authString := r.Header.Get("Authorization")
    if err := checkToken(authString); err != nil {
        msg := fmt.Sprintf("Authentication failure: %v", err)
        http.Error(w, msg, http.StatusUnauthorized)
        return
    }

    var data map[string]interface{}

    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    err = json.Unmarshal(body, &data)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    name := data["name"].(string)
    update, ok := store.values[name]
    if  ok {
        update.Current = data["value"]
    } else {
        update = &Update{Current: data["value"]}
        store.mux.Lock()
        store.values[name] = update
        store.mux.Unlock()
    }

    if min, ok := data["min"]; ok {
        update.Min = min.(float64)
    }

    if max, ok := data["max"]; ok {
        update.Max = max.(float64)
    }

    w.WriteHeader(http.StatusNoContent)
}

func updateData() {
    updateTicker := time.NewTicker(updatePeriod)

    defer func() {
        updateTicker.Stop()
    }()

    sess, err := session.NewSession(&aws.Config{
        Region: aws.String(S3_REGION),
        Credentials: credentials.NewSharedCredentials("", "twitch-plays"),
    })
    if err != nil {
        log.Println(err)
        return
    }

    for {
        <-updateTicker.C

        jsonString, err := json.Marshal(store.values)
        if err != nil {
            log.Println(err)
            return
        }
        
        os.Stdout.Write(jsonString)
        buffer := bytes.NewBuffer(jsonString)

        _, err = s3.New(sess).PutObject(&s3.PutObjectInput{
            Bucket:               aws.String(S3_BUCKET),
            Key:                  aws.String(S3_KEY),
            ACL:                  aws.String("private"),
            Body:                 bytes.NewReader(buffer.Bytes()),
            ContentLength:        aws.Int64(int64(len(buffer.Bytes()))),
            ContentType:          aws.String("application/json"),
            ContentDisposition:   aws.String("attachment"),
            ServerSideEncryption: aws.String("AES256"),
        })
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func main() {
    http.HandleFunc("/update/",  serveIngest)

    go updateData()

    log.Println("Starting TPI Server!")
    if err := http.ListenAndServe(":" + *port, nil); err != nil {
        log.Fatal(err)
    }
}
