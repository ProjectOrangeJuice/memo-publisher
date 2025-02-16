# Project Sync

This system reads webhooks from [usememos](https://www.usememos.com/) and generates a post for [Hugo](https://gohugo.io/). It is a simple way to keep a blog updated with the latest content from usememos.

# Requirements

- usememos needs to be able to reach this server inorder to send the webhook

# Intended use

The intention for the system is to have this server take in the webhook from usememos and generate a page for hugo. It will then push it to git, where a CI (such as code actions) will build and deploy the site. This way, the site will always be up to date with the latest content from usememos.

It will only publish "public" posts. The date of a post will be the date the webhook was sent. So updating a post will overwrite the date.

Only resources that are images are supported

# How to use

1) Deploy the server to a server that can be reached by usememos, here is a kubernetes example
    1a) The git url should have the token attached that gives write access
```yaml

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: projectsync
  namespace: memo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: projectsync
  template:
    metadata:
      labels:
        app: projectsync
    spec:
      containers:
      - name: projectsync
        image: projectorangejuice/projectsync:latest
        env:
        - name: MEMO_URL
            value: "" # Used to fetch resources
        - name: GIT_URL
          valueFrom:
            secretKeyRef:
              name: projectsync
              key: GIT_URL

--- 

apiVersion: v1
kind: Service
metadata:
  name: projectsync
  namespace: memo
spec:
  selector:
    app: projectsync
  ports:
  - protocol: TCP
    port: 8080 
    targetPort: 8080  
  type: ClusterIP

---

apiVersion: v1
data:
  GIT_URL: {{ git url with token }}
kind: Secret
metadata:
  name: projectsync
  namespace: memo
type: Opaque

```

2) Set up the webhook in usememos to point to the server with the address `http://projectsync:8080/webhook` (if you're using the kubernetes service example). This is found in the settings > preferences
    *Make sure there is no spaces after the url, usememos will not send the webhook if there is a space*