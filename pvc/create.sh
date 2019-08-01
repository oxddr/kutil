#!/usr/bin/env bash

start=${1:=1}
end=${2:=10}

create-pvc() {
    local i=$1
    cat <<EOF | kubectl create -f -
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-$i
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-$i
spec:
  selector:
    matchLabels:
      app: backend-$i
  replicas: 1
  template:
    metadata:
      labels:
        app: backend-$i
    spec:
      containers:
      - name: myapp
        image: gcr.io/google-containers/pause:3.1
        volumeMounts:
        - name: pv-$i
          mountPath: /storage
      volumes:
      - name: pv-$i
        persistentVolumeClaim:
          claimName: pvc-$i
EOF
}

for i in $(seq $start $end); do
    create-pvc $i
done
