# Request model with streaming transcription
import requests
import json
import time
import os

model_url = os.getenv("MODEL_URL")
f = open("audio2.mp3", "rb")
while True:
    try:

        url = f"{model_url}/v1/audio/transcriptions"
        host = model_url.split("://")[1].split(":")[0]
        with requests.post(
            url,
            files={"file": f},
            stream=True,
            headers={"Host": host},
        ) as response:

            if response.headers.get("cf-ray"):
                print(f"--Response from Beamlit--")
            else:
                print(
                    f"--Response from Local--\n\n"
                )  # Read the response which is streamed
            if response.status_code == 200:
                for chunk in response.iter_content(
                    chunk_size=1024
                ):  # Adjust chunk_size as needed
                    if chunk:  # Filter out keep-alive new chunks
                        # Process the chunk
                        print(chunk)

        print("\n\n---Sleeping for 1 second---\n\n")
        time.sleep(1)
    except Exception as e:
        print(f"Error: {e}")
        time.sleep(1)
