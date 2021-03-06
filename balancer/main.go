package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var servers = sync.Map{}

func printPod(verb string, pod *v1.Pod) {
	ready := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady {
			ready = (cond.Status == v1.ConditionTrue)
		}
	}
	fmt.Println(time.Now(), verb, pod.Name, pod.Status.Phase, ready)
}

func podIsAlive(pod *v1.Pod) bool {
	ready := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady {
			ready = (cond.Status == v1.ConditionTrue)
		}
	}
	return ready && !podIsTerminating(pod)
}

func podIsTerminating(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func monitorBackends() chan struct{} {
	clientset, err := kubernetes.NewForConfig(&rest.Config{
		Host:            os.Getenv("K8S_MASTER_HOST"),
		BearerToken:     os.Getenv("K8S_MASTER_TOKEN"),
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	})
	if err != nil {
		glog.Errorln(err)
	}

	watchlist := cache.NewFilteredListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		string(v1.ResourcePods),
		"shutdownpoc",
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = "app=server"
		},
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&v1.Pod{},
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				printPod("added", pod)
				servers.Store(pod.Name, pod)
			},
			DeleteFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				printPod("deleted", pod)
				servers.Delete(pod.Name)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				pod := newObj.(*v1.Pod)
				printPod("changed", pod)
				servers.Store(pod.Name, pod)
				spew.Dump(pod.DeletionTimestamp)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop
}

func runBalancer() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a port number!")
		return
	}
	PORT := ":" + arguments[1]

	s, err := net.ResolveUDPAddr("udp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer connection.Close()
	buffer := make([]byte, 1024)

	fmt.Println("Ready...")

	conns := &sync.Map{}

	for {
		n, addr, err := connection.ReadFromUDP(buffer)

		pkt := strings.Split(string(buffer[0:n]), "|")
		pktUUID := pkt[0]

		k := pktUUID + "-" + addr.String()
		fmt.Println("Bridging key", k)

		var bridgeTo *net.UDPAddr
		if v, found := conns.Load(k); found {
			bridgeTo = v.(*net.UDPAddr)
		} else {
			servers.Range(func(key, value interface{}) bool {
				pod := value.(*v1.Pod)
				if podIsAlive(pod) {
					fmt.Println("Found", pod.Status.PodIP)
					var err error
					bridgeTo, err = net.ResolveUDPAddr("udp4", pod.Status.PodIP+":1234")
					if err != nil {
						fmt.Println(err)
						return true
					}
					conns.Store(k, bridgeTo)
					conns.Store(pktUUID+"-"+bridgeTo.String(), addr)
					return false
				}
				return true
			})
		}

		fmt.Println(addr, "->", bridgeTo)
		_, err = connection.WriteToUDP(buffer[0:n], bridgeTo)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func main() {
	fmt.Println("Starting")
	stop := monitorBackends()
	defer close(stop)

	go runBalancer()

	for {
		time.Sleep(1 * time.Second)
		servers.Range(func(key, value interface{}) bool {
			if value.(*v1.Pod).DeletionTimestamp != nil {
				fmt.Println(key, "is terminating")
			}
			return true
		})
	}
}
