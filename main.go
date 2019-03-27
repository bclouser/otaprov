package main

import (
	"fmt"
	"os"
	"path"
	// "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"archive/zip"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

var namespace string = "default"

type FileSource struct {
	Name string
	Body string
}

func main() {
	var filesToZip []FileSource
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Get the gateway-tls secret
	gatewayTlsSecret, err := clientset.CoreV1().Secrets(namespace).Get("gateway-tls", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Found Secret, gateway-tls:\n")
	fmt.Printf("%s\n", gatewayTlsSecret.Data)
	filesToZip = append(filesToZip, FileSource{"server_ca.pem", string(gatewayTlsSecret.Data["server_ca.pem"])})

	// Get the user-keys secret
	userKeysSecret, err := clientset.CoreV1().Secrets(namespace).Get("user-keys", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Found Secret, user-keys:\n")
	fmt.Printf("%s\n", userKeysSecret.Data)


	// I have these two (nearly identical) structs so that i can parse json and 
	// remove the private and public keys respectively. Until I learn a better way.
	type KeyDataPublic struct {
		Keytype string `json:"keytype"`
		Keyval struct {Public string `json:"public"`} `json:"keyval"`
	}
	type KeyDataPrivate struct {
		Keytype string `json:"keytype"`
		Keyval struct {Private string `json:"private"`} `json:"keyval"`
	}

	// NOTE: This should pull directly from keyserver... currently this will stop working on a rollover
	var keyDataPriv []KeyDataPrivate
	var keyDataPub []KeyDataPublic
	err = json.Unmarshal([]byte(userKeysSecret.Data["keys"]), &keyDataPriv)
	if err != nil {
		panic(err.Error())
	}
	err = json.Unmarshal([]byte(userKeysSecret.Data["keys"]), &keyDataPub)
	if err != nil {
		panic(err.Error())
	}

	pubKeyJson, err := json.Marshal(keyDataPub[0])
	if err != nil {
		panic(err.Error())
	}
	privKeyJson, err := json.Marshal(keyDataPriv[0])
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("targets.pub = %s\n", string(pubKeyJson))
	fmt.Printf("targets.sec = %s\n", string(privKeyJson))

	filesToZip = append(filesToZip, FileSource{"targets.pub", string(pubKeyJson)})
	filesToZip = append(filesToZip, FileSource{"targets.sec", string(privKeyJson)})

	// Collect root json directly from the reposerver API
	response, err := http.Get("http://tuf-reposerver/api/v1/user_repo/root.json")
	if err != nil {
		panic(err.Error())
	}
	rootJson, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Got root.json: \n")
	fmt.Printf("%s\n", rootJson)
	filesToZip = append(filesToZip, FileSource{"root.json", string(rootJson)})

	// Create autoprov.url, tufrepo.url, and treehub.json from environment
	dnsName := os.Getenv("DNS_NAME")
	serverName := os.Getenv("SERVER_NAME")
	     var tufrepoStr string = "http://api."+ dnsName +"/repo/"
     var autoprovStr string = "https://"+ serverName +":30443"
	fmt.Printf("tufrepoStr is %s and autoprovStr is %s\n", tufrepoStr, autoprovStr)

	filesToZip = append(filesToZip, FileSource{"autoprov.url", autoprovStr})
	filesToZip = append(filesToZip, FileSource{"tufrepo.url", tufrepoStr})

	// TODO, these ids and secrets should be variable.
	var treehubJson string = `{
    "oauth2": { 
       "server": "http://oauth2.`+ dnsName +`",
         "client_id" : "7a455f3b-2234-43b5-9d13-7d8823494f21",
         "client_secret" : "OTbGcZx6my"
       },
       "ostree": {
           "server": "http://api.`+ dnsName +`/treehub/api/v3/" 
       }
     }`
     filesToZip = append(filesToZip, FileSource{"treehub.json", treehubJson})

    credentialsPath := os.Getenv("CREDENTIALS_DIR")
    // Create the Zip File
	zipFile, err := os.Create(path.Join(credentialsPath, "credentials.zip"))
    if err != nil {
        panic(err.Error())
    }
    defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)

	for _, file := range filesToZip {
		fmt.Printf("Adding " + file.Name + " to archive\n")
		f, err := zipWriter.Create(file.Name)
		if err != nil {
			panic(err.Error())
		}

		_, err = f.Write([]byte(file.Body))
		if err != nil {
			panic(err.Error())
		}
	}

	err = zipWriter.Close()
	if err != nil {
		panic(err.Error())
	}
}