terraform {
  required_providers {
    yandex = {
      source = "yandex-cloud/yandex"
    }
  }
  required_version = ">= 0.13"
}

provider "yandex" {
  zone = "ru-central1-a"
}

data "yandex_compute_image" "coi" {
  family = "container-optimized-image"
}

resource "yandex_compute_instance_group" "catgpt-group" {
  name = "catgpt-ig"

  service_account_id = "ajeoi0k6f2skjqem2mjg"

  allocation_policy {
    zones = ["ru-central1-a", "ru-central1-b"]
  }

  deploy_policy {
    max_expansion   = 1
    max_unavailable = 1
  }

  scale_policy {
    fixed_scale {
      size = 2
    }
  }

  health_check {
    timeout = 1
    tcp_options {
      port = 8888
    }

    interval = 10
  }

  load_balancer {
    target_group_name = "catgpt-target-group"
  }

  instance_template {
    name = "catgpt-{instance.short_id}"
    service_account_id = "ajeoi0k6f2skjqem2mjg"

    resources {
      cores  = 2
      memory = 1
      core_fraction = 5
    }

    scheduling_policy {
      preemptible = true
    }

    boot_disk {
      initialize_params {
        size = 30
        image_id = data.yandex_compute_image.coi.id
      }
    }

    network_interface {
      subnet_ids = ["e9bderrrnte1ch3oemag", "e2l0426dubo33bd986s8"]
      nat = true
    }

    metadata = {
      ssh-keys = "ubuntu:${file("~/.ssh/id_rsa.pub")}"
      docker-container-declaration = file("${path.module}/docker-meta.yml")
    }
  }

}

resource "yandex_lb_network_load_balancer" "catgpt-lb" {
  name = "catgpt-lb"

  listener {
    name = "catgpt-listener"
    port = 80
    target_port = 8888
    external_address_spec {
      ip_version = "ipv4"
    }
  }

  attached_target_group {
    target_group_id = yandex_compute_instance_group.catgpt-group.load_balancer[0].target_group_id

    healthcheck {
      name = "tcp"
      tcp_options {
        port = 8888
      }
    }
  }
}