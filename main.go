package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	protobufserializer "k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

func main() {
	var endpoint, keyFile, certFile, caFile, resFile string
	flag.StringVar(&endpoint, "endpoint", "https://127.0.0.1:2379", "Etcd endpoint.")
	flag.StringVar(&keyFile, "key", "", "TLS client key.")
	flag.StringVar(&certFile, "cert", "", "TLS client certificate.")
	flag.StringVar(&caFile, "cacert", "", "Server TLS CA certificate.")
	flag.StringVar(&resFile, "resfile", "", "A resource manifest.")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify action: dump or ls [<key>] or get <key> or put <key>\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "get" && flag.NArg() == 1 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify <key> for get operation\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "dump" && flag.NArg() != 1 {
		fmt.Fprint(os.Stderr, "ERROR: you cannot specify positional arguments with dump\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "put" && flag.NArg() == 1 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify <key> for put operation\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "put" && len(resFile) == 0 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify \"-resfile\" flag for put operation\n")
		os.Exit(1)
	}
	action := flag.Arg(0)
	key := ""
	if flag.NArg() > 1 {
		key = flag.Arg(1)
	}

	var tlsConfig *tls.Config
	if len(certFile) != 0 || len(keyFile) != 0 || len(caFile) != 0 {
		tlsInfo := transport.TLSInfo{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}
		var err error
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to create client config: %v\n", err)
			os.Exit(1)
		}
	}

	config := clientv3.Config{
		Endpoints:   []string{endpoint},
		TLS:         tlsConfig,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: unable to connect to etcd: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	switch action {
	case "ls":
		err = listKeys(client, key)
	case "get":
		err = getKey(client, key)
	case "dump":
		err = dump(client)
	case "put":
		err = putKey(client, key, resFile)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: invalid action: %s\n", action)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s-ing %s: %v\n", action, key, err)
		os.Exit(1)
	}
}

func listKeys(client *clientv3.Client, key string) error {
	var resp *clientv3.GetResponse
	var err error
	if len(key) == 0 {
		resp, err = clientv3.NewKV(client).Get(context.Background(), "/", clientv3.WithFromKey(), clientv3.WithKeysOnly())
	} else {
		resp, err = clientv3.NewKV(client).Get(context.Background(), key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	}
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		fmt.Println(string(kv.Key))
	}

	return nil
}

func getKey(client *clientv3.Client, key string) error {
	resp, err := clientv3.NewKV(client).Get(context.Background(), key)
	if err != nil {
		return err
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	encoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, true)

	for _, kv := range resp.Kvs {
		obj, _, err := decoder.Decode(kv.Value, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: unable to decode %s: %v\n", kv.Key, err)
			continue
		}
		err = encoder.Encode(obj, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: unable to encode %s: %v\n", kv.Key, err)
			continue
		}
	}

	return nil
}

func putKey(client *clientv3.Client, key, resFile string) error {
	data, err := ioutil.ReadFile(resFile)
	if err != nil {
		return err
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return err
	}

	objProto := &bytes.Buffer{}
	encoder := protobufserializer.NewSerializer(scheme.Scheme, scheme.Scheme)
	err = encoder.Encode(obj, objProto)
	if err != nil {
		return err
	}

	resp, err := clientv3.NewKV(client).Put(context.Background(), key, objProto.String())
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "new revision %d for key %s was created", resp.Header.Revision, key)

	return nil
}

func dump(client *clientv3.Client) error {
	response, err := clientv3.NewKV(client).Get(context.Background(), "/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	if err != nil {
		return err
	}

	kvData := []etcd3kv{}
	decoder := scheme.Codecs.UniversalDeserializer()
	encoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	objJSON := &bytes.Buffer{}

	for _, kv := range response.Kvs {
		obj, _, err := decoder.Decode(kv.Value, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: error decoding value %q: %v\n", string(kv.Value), err)
			continue
		}
		objJSON.Reset()
		if err := encoder.Encode(obj, objJSON); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: error encoding object %#v as JSON: %v", obj, err)
			continue
		}
		kvData = append(
			kvData,
			etcd3kv{
				Key:            string(kv.Key),
				Value:          string(objJSON.Bytes()),
				CreateRevision: kv.CreateRevision,
				ModRevision:    kv.ModRevision,
				Version:        kv.Version,
				Lease:          kv.Lease,
			},
		)
	}

	jsonData, err := json.MarshalIndent(kvData, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))

	return nil
}

type etcd3kv struct {
	Key            string `json:"key,omitempty"`
	Value          string `json:"value,omitempty"`
	CreateRevision int64  `json:"create_revision,omitempty"`
	ModRevision    int64  `json:"mod_revision,omitempty"`
	Version        int64  `json:"version,omitempty"`
	Lease          int64  `json:"lease,omitempty"`
}
