from flask import Flask, request, jsonify, Response, stream_with_context
import whisper
import time
import json
from prometheus_client import (
    Summary,
    Counter,
    generate_latest,
    CONTENT_TYPE_LATEST,
)
import torch

app = Flask(__name__)

# Load the Whisper tiny model
model = whisper.load_model("tiny")

# Metrics for Prometheus
REQUEST_COUNT = Counter(
    "transcription_requests_total", "Total number of transcription requests"
)
TRANSCRIPTION_DURATION = Summary(
    "transcription_duration_seconds", "Duration of transcription requests in seconds"
)


@app.route("/")
def index():
    return jsonify({"message": "OK"})


@app.route("/metrics")
def metrics():
    return Response(generate_latest(), mimetype=CONTENT_TYPE_LATEST)


@app.route("/transcribe", methods=["POST"])
def transcribe():
    REQUEST_COUNT.inc()

    if "file" not in request.files:
        return jsonify({"error": "No file provided"}), 400

    def generate():
        try:
            file = request.files["file"]
            temp_path = "temp_audio.mp3"
            file.save(temp_path)
            print("File saved successfully")

            start_time = time.time()

            yield "data: " + json.dumps(
                {"type": "status", "message": "Processing started"}
            ) + "\n\n"
            print("Status message sent")

            # Load and preprocess audio
            audio = whisper.load_audio(temp_path)
            audio = whisper.pad_or_trim(audio)
            mel = whisper.log_mel_spectrogram(audio).to(model.device)
            print("Audio processed")

            # Detect language
            _, probs = model.detect_language(mel)
            detected_lang = max(probs, key=probs.get)
            yield "data: " + json.dumps(
                {"type": "language", "language": detected_lang}
            ) + "\n\n"
            print(f"Language detected: {detected_lang}")

            # Transcribe
            options = whisper.DecodingOptions(
                language=detected_lang,
                without_timestamps=False,
                fp16=torch.cuda.is_available(),
            )

            print("Starting transcription")
            result = model.decode(mel, options)
            print("Transcription complete")

            # Send the complete transcription as one segment
            yield "data: " + json.dumps(
                {
                    "type": "segment",
                    "text": result.text,
                    "start": 0,
                    "end": len(audio) / whisper.audio.SAMPLE_RATE,
                }
            ) + "\n\n"

            duration = time.time() - start_time
            TRANSCRIPTION_DURATION.observe(duration)

            print("Sending completion message")
            yield "data: " + json.dumps(
                {"type": "complete", "duration": duration, "full_text": result.text}
            ) + "\n\n"

        except Exception as e:
            print(f"Error occurred: {str(e)}")
            yield "data: " + json.dumps({"type": "error", "message": str(e)}) + "\n\n"
        finally:
            print("Stream complete")

    return Response(
        stream_with_context(generate()),
        mimetype="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
            "Transfer-Encoding": "chunked",
        },
    )


if __name__ == "__main__":
    # Run the Flask app, which now also serves metrics
    app.run(host="0.0.0.0", port=5000)
