FROM jenkins/slave

<<<<<<< HEAD
RUN apt-get install sudo -y
=======
RUN apt-get update && \ apt-get -y install sudo
>>>>>>> 471fdea01aa8362d01e070ca68bd11615e1164a7
RUN sudo apt-get update
RUN sudo apt install docker.io
RUN sudo systemctl start docker
RUN sudo systemctl enable docker
