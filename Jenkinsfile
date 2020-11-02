//api-getway
pipeline {
  agent any
    stages {
      stage('get_commit_msg') {
        steps {
          script {
            env.GIT_COMMIT_MSG = sh (script: 'git log -1 --pretty=%B ${GIT_COMMIT}', returnStdout: true).trim()
            env.GIT_SHORT_COMMIT = sh(returnStdout: true, script: "git log -n 1 --pretty=format:'%h'").trim()
            env.GIT_COMMITTER_EMAIL = sh (script: "git --no-pager show -s --format='%ae'", returnStdout: true  ).trim()
            env.GIT_REPO_NAME = scm.getUserRemoteConfigs()[0].getUrl().tokenize('/')[3].split("\\.")[0]
            echo 'drivehub.azurecr.io/meateam/'+env.GIT_REPO_NAME+':master_'+env.GIT_SHORT_COMMIT
          }
        }
      }
      // build image for unit test
      stage('build dockerfile of tests') {
        steps {
            sh "docker build -t unittest -f test.Dockerfile ." 
        }  
      }
      // build image of system wheb pushed to master or develop
      stage('build image') {
        when {
            anyOf {
                branch 'master'; branch 'develop'
            }  
        }
        parallel {
          // when pushed to master or develop build image and push to acr 
          stage('build dockerfile of system only for master and develop and push them to acr') {
           steps {
              script{
                if(env.GIT_BRANCH == 'master') {
                  sh "docker build -t  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:master_${env.GIT_SHORT_COMMIT} ."
                  sh "docker push  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:master_${env.GIT_SHORT_COMMIT}"
                }
                else if(env.GIT_BRANCH == 'develop') {
                  sh "docker build -t  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:develop ."
                  sh "docker push  drivehub.azurecr.io/meateam/${env.GIT_REPO_NAME}:develop"  
                }
              }  
            }
            post {
              always {
                discordSend description: '**service**: '+ env.GIT_REPO_NAME + '\n **Build**:' + " " + env.BUILD_NUMBER + '\n **Branch**:' + " " + env.GIT_BRANCH + '\n **Status**:' + " " +  currentBuild.result + '\n \n \n **Commit ID**:'+ " " + env.GIT_SHORT_COMMIT + '\n **commit massage**:' + " " + env.GIT_COMMIT_MSG + '\n **commit email**:' + " " + env.GIT_COMMITTER_EMAIL, footer: '', image: '', link: 'http://jnk-devops-ci-cd.northeurope.cloudapp.azure.com/blue/organizations/jenkins/'+env.JOB_FOR_URL+'/detail/'+env.BRANCH_FOR_URL+'/'+env.BUILD_NUMBER+'/pipeline', result: currentBuild.result, thumbnail: '', title: 'Logs build dockerfile master/develop', webhookURL: env.discord   
              }
            }
          }
          // login to acr when pushed to branch master or develop
          stage('login to azure container registry') {
            steps {  
              withCredentials([usernamePassword(credentialsId:'DRIVE_ACR',usernameVariable: 'USER', passwordVariable: 'PASS')]) {
                sh "docker login  drivehub.azurecr.io -u ${USER} -p ${PASS}"
              }  
            }
          }
        }
       }
    }   
}