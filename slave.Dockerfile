FROM jenkins/slave

RUN apt-get install sudo -y
RUN sudo apt-get update
RUN sudo apt install docker.io
RUN sudo systemctl start docker
RUN sudo systemctl enable docker
