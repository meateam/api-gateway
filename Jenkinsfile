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
            echo 'drivehub.azurecr.io/'+env.GIT_REPO_NAME+'/master:'+env.GIT_SHORT_COMMIT
            env.isr = sh "sed -i 'drive/%2Fapi-gateway/'"${JOb_NAME}"'/'"
            echo env.isr
          }
        }
      }
      stage('build image of test and system') {
        parallel {
          stage('build dockerfile of tests') {
            steps {
              sh "docker build -t unittest -f test.Dockerfile ." 
            }  
          }
          stage('login to azure container registry') {
            steps{  
              withCredentials([usernamePassword(credentialsId:'Drive_ACR',usernameVariable: 'USER', passwordVariable: 'PASS')]) {
                sh "docker login  drivehub.azurecr.io -u ${USER} -p ${PASS}"
              }
            }
          }  
          stage('build dockerfile of system only for master and develop') {
            when {
              anyOf {
                 branch 'master'; branch 'develop'
              }
            }
            steps {
              script{
                if(env.GIT_BRANCH == 'master') {
                  sh "docker build -t  drivehub.azurecr.io/${env.GIT_REPO_NAME}/master:${env.GIT_SHORT_COMMIT} ."
                  sh "docker push  drivehub.azurecr.io/${env.GIT_REPO_NAME}/master:${env.GIT_SHORT_COMMIT}"
                }
                else if(env.GIT_BRANCH == 'develop') {
                  sh "docker build -t  drivehub.azurecr.io/${env.GIT_REPO_NAME}/develop ."
                  sh "docker push  drivehub.azurecr.io/${env.GIT_REPO_NAME}/develop"  
                }
              } 
            }
          }     
        }
      }  
      stage('run unit tests') {   
        steps {
          sh "docker run unittest"  
        }
        post {
          always {
            discordSend description: '**service**: '+ env.GIT_REPO_NAME + '\n **Build**:' + " " + env.BUILD_NUMBER + '\n **Branch**:' + " " + env.GIT_BRANCH + '\n **Status**:' + " " +  currentBuild.result + '\n \n \n **Commit ID**:'+ " " + env.GIT_SHORT_COMMIT + '\n **commit massage**:' + " " + env.GIT_COMMIT_MSG + '\n **commit email**:' + " " + env.GIT_COMMITTER_EMAIL, footer: '', image: '', link: env.BUILD_URL + 'consoleText', result: currentBuild.result, thumbnail: '', title: ' link to result', webhookURL: 'https://discord.com/api/webhooks/735056754051645451/jYad6fXNkPMnD7mopiCJx2qLNoXZnvNUaYj5tYztcAIWQCoVl6m2tE2kmdhrFwoAASbv'   
          }
        }
      }
    }   
}