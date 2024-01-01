import com.bmuschko.gradle.docker.tasks.image.DockerBuildImage
import com.bmuschko.gradle.docker.tasks.image.DockerPushImage

/*
 * This file was generated by the Gradle 'init' task.
 *
 * This is a general purpose Gradle build.
 * To learn more about Gradle by exploring our Samples at https://docs.gradle.org/8.3/samples
 */

plugins {
    id("base")
    id("com.fussionlabs.gradle.go-plugin") version("0.5.4")
    id("com.bmuschko.docker-remote-api") version("9.4.0")
}

version = "1.0.0"

val operatorName = "configmap-replicator"
val imageRepository = "dm0275/configmap-replicator"
val imageTags = listOf("$imageRepository:latest", "$imageRepository:$version")

go {
    os = listOf("linux")
    arch = listOf("amd64")
}

docker {
    registryCredentials {
        username = System.getenv("DOCKER_USERNAME")
        password = System.getenv("DOCKER_PASSWORD")
    }
}

tasks.register("dockerBuildImage", DockerBuildImage::class) {
    dependsOn("build")
    inputDir.set(rootDir)
    images.addAll(imageTags)
}

tasks.register("dockerPushImage", DockerPushImage::class.java) {
    dependsOn(tasks.getByPath("dockerBuildImage"))
    images.addAll(imageTags)
}
