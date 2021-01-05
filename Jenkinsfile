//api-getway
pipeline {
  agent {    
       kubernetes {
       defaultContainer 'dind-slave'  
       yaml """
      apiVersion: v1 
      kind: Pod 
      metadata: 
          name: k8s-worker
      spec: 
          containers: 
            - name: dind-slave
              image: aymdev/dind-compose 
              resources: 
                  requests: 
                      cpu: 20m 
                      memory: 512Mi 
              securityContext: 
                  privileged: true 
              volumeMounts: 
                - name: docker-graph-storage 
                  mountPath: /var/lib/docker
            - name: kube-helm-slave
              image:  qayesodot/slave-jenkins:kube-helm
              securityContext:
                allowPrivilegeEscalation: false
                runAsUser: 0
              command: ["/bin/sh"]
              args: ["-c","while true; do echo hello; sleep 10;done"]            
          volumes: 
            - name: docker-graph-storage 
              emptyDir: {}
 """
    }
  }   
  stages {
      // this stage create enviroment variable from git for discored massage
      stage('get_commit_msg') {
        steps {
          container('jnlp'){
          script {
            env.GIT_COMMIT_MSG = sh (script: 'git log -1 --pretty=%B ${GIT_COMMIT}', returnStdout: true).trim()
            env.GIT_SHORT_COMMIT = sh(returnStdout: true, script: "git log -n 1 --pretty=format:'%h'").trim()
            env.GIT_COMMITTER_EMAIL = sh (script: "git --no-pager show -s --format='%ae'", returnStdout: true  ).trim()
            env.GIT_REPO_NAME = scm.getUserRemoteConfigs()[0].getUrl().tokenize('/')[3].split("\\.")[0]

            // Takes the branch name and replaces the slashes with the %2F mark 
            env.BRANCH_FOR_URL = sh([script: "echo ${GIT_BRANCH} | sed 's;/;%2F;g'", returnStdout: true]).trim()
            // Takes the job path variable and replaces the slashes with the %2F mark 
            env.JOB_PATH = sh([script: "echo ${JOB_NAME} | sed 's;/;%2F;g'", returnStdout: true]).trim()
            // creating variable that contain the job path without the branch name  
            env.JOB_WITHOUT_BRANCH = sh([script: "echo ${env.JOB_PATH} | sed 's;${BRANCH_FOR_URL};'';g'", returnStdout: true]).trim() 
            //  creating variable that contain the JOB_WITHOUT_BRANCH variable without the last 3 characters 
            env.JOB_FOR_URL = sh([script: "echo ${JOB_WITHOUT_BRANCH}|rev | cut -c 4- | rev", returnStdout: true]).trim()  
            echo "${env.JOB_FOR_URL}" 
          }
        }
      }
    } 

      // run unit test using docker-compose
      stage('run unit tests') {   
        steps {
          withCredentials([usernamePassword(credentialsId:'DRIVE_ACR',usernameVariable: 'USER', passwordVariable: 'PASS')]) {
                sh "docker login drivehub.azurecr.io -u ${USER} -p ${PASS}"
          }
          configFileProvider([configFile(fileId:'d9e51ae8-06c8-4dc4-ba0d-d4794033bddd',variable:'API_CONFIG_FILE')]){
            sh "cp ${env.API_CONFIG_FILE} ./kdrive.env" 
            sh "cat kdrive.env"
            sh "docker-compose -f docker-compose.test.yaml up  --build --force-recreate --renew-anon-volumes --exit-code-from api-gateway" 
            sh "rm kdrive.env" 
          } 
        }
        // post {
        //   always {
        //     discordSend description: '**service**: '+ env.GIT_REPO_NAME + '\n **Build**:' + " " + env.BUILD_NUMBER + '\n **Branch**:' + " " + env.GIT_BRANCH + '\n **Status**:' + " " +  currentBuild.result + '\n \n \n **Commit ID**:'+ " " + env.GIT_SHORT_COMMIT + '\n **commit massage**:' + " " + env.GIT_COMMIT_MSG + '\n **commit email**:' + " " + env.GIT_COMMITTER_EMAIL, footer: '', image: '', link: 'http://jnk-devops-ci-cd.northeurope.cloudapp.azure.com/blue/organizations/jenkins/'+env.JOB_FOR_URL+'/detail/'+env.BRANCH_FOR_URL+'/'+env.BUILD_NUMBER+'/pipeline', result: currentBuild.result, thumbnail: '', title: ' link to logs of unit test', webhookURL: env.discord   
        //   }
        // }
      }
      // build images unit tests and system
      // stage('build image of test and system') {
      //   parallel {
      //     // build image of unit test 
      //     stage('build dockerfile of tests') {
      //       steps {
              
      //         sh "docker build -t unittest -f test.Dockerfile ."
               
      //       }  
      //     }
      //     // login to acr when pushed to branch master or develop 
      //     stage('login to azure container registry') {
      //       when {
      //         anyOf {
      //            branch 'master'; branch 'develop'
      //         }
      //       }
      //       steps{  
      //         withCredentials([usernamePassword(credentialsId:'DRIVE_ACR',usernameVariable: 'USER', passwordVariable: 'PASS')]) {
      //           sh "docker login drivehub.azurecr.io -u ${USER} -p ${PASS}"
      //         }
      //       }
      //     }  
      //     // when pushed to master or develop build image and push to acr
      //     stage('build dockerfile of system only for master and develop') {
      //       when {
      //         anyOf {
      //            branch 'master'; branch 'develop'
      //         }
      //       }
      //       steps {
      //         script {
      //           if(env.GIT_BRANCH == 'master') {  
      //             sh "docker build -t drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:master ."
      //             sh "docker push  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:master"
      //           }
      //           else if(env.GIT_BRANCH == 'develop') {
      //              sh "docker build -t  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:develop ."
      //              sh "docker push  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:develop"  
      //           }
      //         }
      //       }  
      //       post {
      //         always {
      //           discordSend description: '**service**: '+ env.GIT_REPO_NAME + '\n **Build**:' + " " + env.BUILD_NUMBER + '\n **Branch**:' + " " + env.GIT_BRANCH + '\n **Status**:' + " " +  currentBuild.result + '\n \n \n **Commit ID**:'+ " " + env.GIT_SHORT_COMMIT + '\n **commit massage**:' + " " + env.GIT_COMMIT_MSG + '\n **commit email**:' + " " + env.GIT_COMMITTER_EMAIL, footer: '', image: '', link: 'http://jnk-devops-ci-cd.northeurope.cloudapp.azure.com/blue/organizations/jenkins/'+env.JOB_FOR_URL+'/detail/'+env.BRANCH_FOR_URL+'/'+env.BUILD_NUMBER+'/pipeline', result: currentBuild.result, thumbnail: '', title: 'Logs build dockerfile master/develop', webhookURL: env.discord    
      //         }
      //       }
      //     }
      //   }     
      // }

    // ---- CD section ---- 
    // stage('create nameSpace,secrets and configMap in the cluster') {
    //     when {
    //       anyOf {
    //         branch 'master'; branch 'develop'
    //       }
    //     }
    //     steps {
    //       container('kube-helm-slave'){
    //         // sh("kubectl get ns develop || kubectl create ns develop")
    //         sh("kubectl get ns ${env.BRANCH_NAME} || kubectl create ns ${env.BRANCH_NAME}")
    //         sleep(10)
    //       script {
    //         if(env.BRANCH_NAME == 'devops/ci') {
    //           configFileProvider([configFile(fileId:'34e71bc6-8b5d-4e31-8d6e-92d991802dcb',variable:'MASTER_CONFIG_FILE')]){
    //           sh ("kubectl apply -f ${env.MASTER_CONFIG_FILE}")  

    //              sh ("kubectl get secrets acr-secret --namespace ${env.BRANCH_NAME} || kubectl create secret docker-registry acr-secret --docker-username=DriveHub --docker-password= Eq0186MYP7hm/bkntY=YW8NpbMhy3PpC  --docker-server=https://drivehub.azurecr.io --namespace ${env.BRANCH_NAME}")
    //           }    
    //         }
    //         else{
    //            configFileProvider([configFile(fileId:'abda1ce7-3925-4759-88a7-5163bdb44382',variable:'DEVELOP_CONFIG_FILE')]){
    //                sh ("kubectl apply -f ${env.DEVELOP_CONFIG_FILE}")  

    //                sh ("kubectl get secrets acr-secret --namespace ${env.BRANCH_NAME} || kubectl create secret docker-registry acr-secret --docker-username=DriveHub --docker-password= Eq0186MYP7hm/bkntY=YW8NpbMhy3PpC  --docker-server=https://drivehub.azurecr.io --namespace ${env.BRANCH_NAME}")
    //           }
    //         }
    //       }
    //     }
    //   }
    // }


    // stage('create and configure ingress under current namespace'){
    //    when {
    //      anyOf {
    //       branch 'master'; branch 'develop'
    //     }
    //   }
    //   steps{
    //     container('kube-helm-slave'){
    //       script {
    //        if(env.BRANCH_NAME == 'master'){ 
    //          sh([script: """
    //          kubectl get deployment --namespace master | grep  ingress-master || (helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx && 
    //          helm repo update && 
    //          helm install --name ingress-master ingress-nginx/ingress-nginx --namespace master \
    //          --set controller.replicaCount=2 --set controller.nodeSelector."beta\\.kubernetes\\.io/os"=linux \
    //          --set defaultBackend.nodeSelector."beta\\.kubernetes\\.io/os"=linux --set controller.service.loadBalancerIP=20.54.101.163)
    //         """])
    //        } else {
    //          sh([script: """
    //          kubectl get deployment --namespace develop | grep  ingress-develop || (helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx && 
    //          helm repo update && 
    //          helm install --name ingress-develop ingress-nginx/ingress-nginx --namespace develop \
    //          --set controller.replicaCount=2 --set controller.nodeSelector."beta\\.kubernetes\\.io/os"=linux \
    //          --set defaultBackend.nodeSelector."beta\\.kubernetes\\.io/os"=linux --set controller.service.loadBalancerIP=51.104.179.70)
    //         """])
    //        }
    //       }
    //     }
    //   }
    // }
    // stage('clone kd-helm reposetory and inject imagePullSecrets block, and replace image tag in common/deployments file'){
    //   when {
    //     anyOf {
    //       branch 'master'; branch 'develop'
    //     }
    //   }
    //   steps {
    //      container('jnlp'){
    //       git branch: 'master',
    //         credentialsId: 'gitHubToken',
    //         url: 'https://github.com/meateam/kd-helm.git'
    //         sh 'cat common/templates/_deployment.yaml'
    //     script {
    //         env.space1 = "- name: acr-secret"
    //         env.space2 = "imagePullSecrets:"
    //     }
    //       sh "sed -i '29 i 2345678      ${env.space2}' ./common/templates/_deployment.yaml && sed -i 's;2345678;'';g' ./common/templates/_deployment.yaml"
    //       sh "sed -i '30 i 2345678        ${env.space1}' ./common/templates/_deployment.yaml && sed -i 's;2345678;'';g' ./common/templates/_deployment.yaml" 
    //       sh "sed -i 's;{{ .Values.image.tag }};${env.BRANCH_TAG_NAME};g' ./common/templates/_deployment.yaml"
    //       sh 'cat common/templates/_deployment.yaml'
    //     }
    //   }
    //   post {
    //     always {
    //         stash includes: '../kd-helm/**/*', name: 'kdHelmRepo'
    //     } 
    //   }
    // }

    // // this stage update the helm-chart packages and deploy or upgrade the drive-app to k8s , depends if drive-app is already deployed on the cluster 
    //   stage('deploy app'){
    //     when {
    //       anyOf {
    //         branch 'master'; branch 'develop'
    //       }
    //     }
    //     steps {
    //       container('kube-helm-slave'){
    //         unstash 'kdHelmRepo'
    //       if(env.BRANCH_NAME == 'master'){ 
    //          sh([script: """
    //          (helm get drive-master && ./helm-dep-up-umbrella.sh ./helm-chart/ && helm upgrade drive-master ./helm-chart/ ) 
    //          ||(./helm-dep-up-umbrella.sh ./helm-chart/ && helm install ./helm-chart/ --name drive-master --namespace master --set global.ingress.hosts[0]=drive-master.northeurope.cloudapp.azure.com)
    //         """])
    //       }
    //       else {
    //          sh([script: """
    //          (helm get drive-develop && ./helm-dep-up-umbrella.sh ./helm-chart/ && helm upgrade drive-develop ./helm-chart/ ) 
    //          ||(./helm-dep-up-umbrella.sh ./helm-chart/ && helm install ./helm-chart/ --name drive-develop --namespace develop --set global.ingress.hosts[0]=drive-develop.northeurope.cloudapp.azure.com)
    //         """])
    //       }
    //       sh "apk --no-cache add curl && curl -I drive-${env.BRANCH_NAME}.northeurope.cloudapp.azure.com/"
    //     }
    //   }
    // }
  }
}   
