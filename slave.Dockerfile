FROM jenkins/slave

RUN apt-get update && \ apt-get -y install sudo
RUN sudo apt-get update
RUN sudo apt install docker.io
RUN sudo systemctl start docker
RUN sudo systemctl enable docker
