apiVersion: v1
kind: Pod
metadata:
  name: slave
  labels:
    jenkins: slave
spec:
  securityContext:
    windowsOptions:
      runAsUserName: "ContainerUser"
  containers:
  - name: jenkins-slave-yosef
    image: docker:dind
    command: [ "sh", "-c", "--" ]
    args: [ "while true; do sleep 30; done;" ] 
    env:
    DOCKER_HOST: tcp://localhost:2375
  hostNetwork: true
  dnsPolicy: Default