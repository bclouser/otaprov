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
	"os/exec"
	"bytes"
	"errors"
	"github.com/satori/go.uuid"
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

var dataPath string
var devicesDir string
var namespace string = "default"

type FileSource struct {
	Name string
	Body string
}


type Device struct {
	Uuid string
	ClientPrivKey string
	ClientCertPem string
	ServerRootCA string
}

type Credentials struct {
	ServerRootCA string
	TargetsPub string
	TargetsSec string
	RootJson string
	AutoProvUrl string
	TufRepoUrl string
	TreeHubJson string
}


/*
 *  Implementation methods
 */

func createCredentials() (Credentials, error) {
	var creds Credentials
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}

	// Get the gateway-tls secret
	gatewayTlsSecret, err := clientset.CoreV1().Secrets(namespace).Get("gateway-tls", metav1.GetOptions{})
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}
	fmt.Printf("Found Secret, gateway-tls:\n")
	fmt.Printf("%s\n", gatewayTlsSecret.Data)

	creds.ServerRootCA = string(gatewayTlsSecret.Data["server_ca.pem"])

	// Get the user-keys secret
	userKeysSecret, err := clientset.CoreV1().Secrets(namespace).Get("user-keys", metav1.GetOptions{})
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
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
		fmt.Printf(err.Error())
		return creds, err
	}
	err = json.Unmarshal([]byte(userKeysSecret.Data["keys"]), &keyDataPub)
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}

	pubKeyJson, err := json.Marshal(keyDataPub[0])
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}
	privKeyJson, err := json.Marshal(keyDataPriv[0])
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}

	fmt.Printf("targets.pub = %s\n", string(pubKeyJson))
	fmt.Printf("targets.sec = %s\n", string(privKeyJson))

	creds.TargetsPub = string(pubKeyJson)
	creds.TargetsSec = string(privKeyJson)

	// Collect root json directly from the reposerver API
	response, err := http.Get("http://tuf-reposerver/api/v1/user_repo/root.json")
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}
	rootJson, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf(err.Error())
		return creds, err
	}
	fmt.Printf("Got root.json: \n")
	fmt.Printf("%s\n", rootJson)
	creds.RootJson = string(rootJson)

	// Create autoprov.url, tufrepo.url, and treehub.json from environment
	dnsName := os.Getenv("DNS_NAME")
	autoProvServer := os.Getenv("AUTOPROV_SERVER_NAME")

	creds.AutoProvUrl = "https://"+ autoProvServer +":30443"
	creds.TufRepoUrl = "http://api."+ dnsName +"/repo/"
	fmt.Printf("tufrepoStr is %s and autoprovStr is %s\n", creds.TufRepoUrl, creds.AutoProvUrl)

	// TODO, these ids and secrets should be variable.
	creds.TreeHubJson = `{
    "oauth2": { 
       "server": "http://oauth2.`+ dnsName +`",
         "client_id" : "7a455f3b-2234-43b5-9d13-7d8823494f21",
         "client_secret" : "OTbGcZx6my"
       },
       "ostree": {
           "server": "http://api.`+ dnsName +`/treehub/api/v3/" 
       }
     }`
    return creds, err
}

func createDevice() (Device, error) {
  // Poke Kubernetes API and attain device's ca.key and ca.rt from gateway-tls-secret
  // creates the in-cluster config

  	var dev Device
  	var serverRootCA string

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

	fmt.Printf("device ca.key: %s\n", string(gatewayTlsSecret.Data["ca.key"]))
	fmt.Printf("device ca.crt: %s\n", string(gatewayTlsSecret.Data["ca.crt"]))

	serverRootCA = string(gatewayTlsSecret.Data["server_ca.pem"])
	fmt.Printf("server_ca.pem: %s\n", serverRootCA)

	// Save these to the filesystem
	os.Mkdir(devicesDir, 0755)
	fmt.Printf("Saving certificate to path: %s\n", devicesDir)
	file := path.Join(devicesDir, "ca.key")
	err = ioutil.WriteFile(file, []byte(gatewayTlsSecret.Data["ca.key"]), 0755)
	if err != nil {
		fmt.Printf("Failed to write ca.key")
		panic(err.Error())
	}
	file = path.Join(devicesDir, "ca.crt")
	err = ioutil.WriteFile(file, []byte(gatewayTlsSecret.Data["ca.crt"]), 0755)
	if err != nil {
		fmt.Printf("Failed to write ca.crt")
		panic(err.Error())
	}

	id := uuid.Must(uuid.NewV4())
	fmt.Printf("UUIDv4: %s\n", id)

	// Append the first 8 bytes of the uuid to the device-id to keep it unique
	uuidText,_ := id.MarshalText()
	var deviceId string = "test-device-id-"+string(uuidText[:8])
	fmt.Printf("deviceId: %s\n", deviceId)

	// Run script to generate the necessary certs
	cmd := exec.Command("/bin/bash", "/usr/local/bin/create-device.sh", id.String(), deviceId)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()

	fmt.Printf("Output: %s\n", out.String())

	if err != nil {
		fmt.Printf("running create-device.sh script failed with err: %s\n", err.Error())
		return dev, err
	}
	fmt.Printf("Script ran Successfully\n")


	pkeyPem,_ := ioutil.ReadFile(path.Join(devicesDir, id.String(), "pkey.pem"))
	clientPem,_ := ioutil.ReadFile(path.Join(devicesDir, id.String(), "client.pem"))
	// Register device with OTA backend
	// RESP_UUID=$(http --ignore-stdin --verify=no PUT "${device_registry}/api/v1/devices" credentials=@"${device_dir}/client.pem" \
	//   deviceUuid="${DEVICE_UUID}" deviceId="${device_id}" deviceName="${device_id}" deviceType=Other "${KUBE_AUTH}")
	//echo "The Device Registry responded with a UUID OF: ${RESP_UUID}"

	type RequestBody struct {
		Credentials string `json:"credentials"`
		DeviceUUID string `json:"deviceUuid"`
		Uuid string `json:"uuid"`
		DeviceId string `json:"deviceId"`
		DeviceName string `json:"deviceName"`
		DeviceType string `json:"deviceType"`
	}

	requestBody := RequestBody{string(clientPem), id.String(), id.String(), deviceId, deviceId, "Other"}
	var jsonBody []byte
	jsonBody, err = json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("Failed to Marshal json\n");
		return dev, err
	}

	fmt.Printf("Here is the json string: %s\n", string(jsonBody))

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, "http://device-registry/api/v1/devices", bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Printf("Failed to create put request\n")
		return dev, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed PUT Request to the device-registry\n")
		return dev, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	if resp.StatusCode != 200 {
		return dev, errors.New("http response status code not ok. Expected 200. got " + resp.Status)
	}

	fmt.Printf("Response body from PUT request was:\n %s\n", buf.String())

	dev.Uuid=id.String()
	dev.ClientPrivKey=string(pkeyPem)
	dev.ClientCertPem=string(clientPem)
	dev.ServerRootCA=serverRootCA

	  // return payload to device for provisioning.
  return dev, nil
}





/*
 *  HTTP server router handler functions and helpers
 */

func createZip(itemsToZip []FileSource, zipBuf *bytes.Buffer) error {
	zipWriter := zip.NewWriter(zipBuf)

	// put all the files into the zip file
	for _, fileToZip := range itemsToZip {
		fmt.Printf("Adding %s to archive\n", fileToZip.Name)

		zipFile, err := zipWriter.Create(fileToZip.Name)
		if err != nil {
			return err
		}

		_, err = zipFile.Write([]byte(fileToZip.Body))
		if err != nil {
			return err
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return err
	}
	return nil
}

func handleCredentialsZip(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("getCredentialsZip\n")
	credentials, err := createCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	var filesToZip []FileSource
	filesToZip = append(filesToZip, FileSource{"root.crt", credentials.ServerRootCA})
	filesToZip = append(filesToZip, FileSource{"targets.pub", credentials.TargetsPub})
	filesToZip = append(filesToZip, FileSource{"targets.sec", credentials.TargetsSec})
	filesToZip = append(filesToZip, FileSource{"root.json", credentials.RootJson})
	filesToZip = append(filesToZip, FileSource{"autoprov.url", credentials.AutoProvUrl})
	filesToZip = append(filesToZip, FileSource{"tufrepo.url", credentials.TufRepoUrl})
	filesToZip = append(filesToZip, FileSource{"treehub.json", credentials.TreeHubJson})

	var zipBuf bytes.Buffer
	err = createZip(filesToZip, &zipBuf)
	if err != nil {
		http.Error(w, "ERROR: Failed to create credentials.zip: " + err.Error(),
					http.StatusInternalServerError)
		return
	}

	// Finally send the zip response
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"credentials.zip\""))
	w.Write(zipBuf.Bytes())
}

func handleNewDevice(w http.ResponseWriter, r *http.Request) {
	device, err := createDevice()
	if err != nil {
		http.Error(w, "ERROR: Failed to create newDevice: " + err.Error(), http.StatusInternalServerError)
		return
	}

	var filesToZip []FileSource
	filesToZip = append(filesToZip, FileSource{"client.pem", device.ClientCertPem})
	filesToZip = append(filesToZip, FileSource{"pkey.pem", device.ClientPrivKey})
	filesToZip = append(filesToZip, FileSource{"root.crt", device.ServerRootCA})

	var zipBuf bytes.Buffer
	err = createZip(filesToZip, &zipBuf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	fmt.Printf("Device uuid is %s\n", device.Uuid)

	// Finally send the zip response
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"certs.zip\""))
	w.Write(zipBuf.Bytes())	
}

func main() {
	dataPath = os.Getenv("DATA_PATH")
	devicesDir = path.Join(dataPath, "/devices")
	http.HandleFunc("/credentials.zip", handleCredentialsZip)
	http.HandleFunc("/create-device", handleNewDevice)
	fmt.Printf("Listening on port 8000\n")
	http.ListenAndServe(":8000", nil)
}
