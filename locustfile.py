from locust import HttpUser, task, between
import random
import string

class KVCacheUser(HttpUser):
    wait_time = between(1, 3)

    @task(3)
    def put_request(self):
        key = ''.join(random.choices(string.ascii_letters, k=10))
        value = ''.join(random.choices(string.ascii_letters, k=10))
        self.client.post("/put", json={"key": key, "value": value})

    @task(7)
    def get_request(self):
        key = "test"
        self.client.get(f"/get?key={key}")
