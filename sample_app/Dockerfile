# Use the official Python image from Docker Hub
FROM python:3.9-slim

# Set the working directory
WORKDIR /app

# Copy the requirements file into the container
COPY requirements.txt .

# Install git
RUN apt-get update && apt-get install -y git

# Install the required Python packages
RUN pip install --no-cache-dir -r requirements.txt

RUN apt-get install -y ffmpeg

# Copy the application code into the container
COPY app.py .

# Expose the ports the app runs on
EXPOSE 5000
EXPOSE 8000

# Run the application
CMD ["python", "app.py"]
