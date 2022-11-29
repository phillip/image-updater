package cmd

import (
	"strconv"
	"strings"
	"testing"
)

var case1 = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world-deployment
spec:
  selector:
    matchLabels:
      app: hello-world
  replicas: 2
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
      - name: hello-world
        image: hello-world:v0.1
        ports:
        	- containerPort: 80`

var case2 = `apiVersion: apps/v1
kind: Deployment
metadata:
	name: hello-world-deployment
spec:
	selector:
	matchLabels:
		app: hello-world
	replicas: 2
	template:
	metadata:
		labels:
		app: hello-world
	spec:
		containers:
		- name: hello-world1
			image: hello-world1:v0.1
			ports:
				- containerPort: 80
		- name: hello-world2
			image: hello-world2:v0.1	
		- name: hello-world3
			image: hello-world3:v0.1`

var case3 = `helloworld:
namespace: default
image:
  name: hello-world
  tag: latest
  pullPolicy: IfNotPresent`

func TestTagReplace(t *testing.T) {
	tag := "v0.2"
	ans := replaceTag(case1, "image", tag, 1)
	if !strings.Contains(ans, "hello-world:"+tag) {
		t.Errorf("Tag not replaced with %s", tag)
	}
}

func TestNth(t *testing.T) {
	tag := "v0.3"
	nth := 2
	ans := replaceTag(case2, "image", tag, 2)
	if !strings.Contains(ans, "hello-world"+strconv.Itoa(nth)+":"+tag) {
		t.Errorf("Tag %d not replaced with %s", nth, tag)
	}
}

func TestIsolatedTag(t *testing.T) {
	tag := "v0.4"
	tagName := "tag"
	ans := replaceTag(case3, tagName, tag, 1)
	if !strings.Contains(ans, tagName+": "+tag) {
		t.Errorf("Isolated tag not replaced with %s", tag)
	}
}
