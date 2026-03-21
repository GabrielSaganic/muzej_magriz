# Magriž Heritage Museum

A digital platform for the "Zavičajna udruga Magriž Martinščića" (Magriž Homeland Association), designed to showcase the cultural heritage, history, and sustainable development of the island of Cres.

## 🚀 Features

-   **Multimedia Feed**: Dynamic display of high-resolution images and videos.
-   **Video Support**: Native full-screen video playback.
-   **Multilingual**: Full support for Croatian, English, Italian, and German.
-   **Modern Design**:
    *   Elegant parallax effects on the home page.
    *   Glassmorphism UI elements.
    *   Enhanced typography (Poppins font).
-   **Mobile Optimization**: Fully responsive interface, optimized for modern smartphones (including Pixel 7).
-   **Rich Descriptions**: Support for HTML formatting in detailed object descriptions (headings, bold text, lists).

## 🛠️ Technologies

-   **Backend**: Go (Golang)
-   **Database**: PostgreSQL
-   **Frontend**: HTML5, CSS3 (Vanilla), JavaScript (Vanilla)
-   **Infrastructure**: Docker & Docker Compose

## 🏁 How to Start the Website

The easiest way to run the project is using **Docker Compose**.

### Prerequisites
-   [Docker](https://www.docker.com/) installed
-   [Docker Compose](https://docs.docker.com/compose/) installed

### Steps to Run

1.  **Clone or download the project** to a folder of your choice.
2.  **Open a terminal** in that folder.
3.  **Start the system** with the following command:
    ```bash
    docker-compose up --build
    ```
4.  **Access the application**:
    *   **Website**: [http://localhost:8080](http://localhost:8080)
    *   **Database Management (Adminer)**: [http://localhost:8081](http://localhost:8081)

## 🗄️ Database Structure

The system uses three main tables:
-   `slike` (images): Stores images, gallery codes, and translated descriptions.
-   `videa` (videos): Stores video URLs, thumbnails, and descriptions.
-   `linkovi` (links): Connects media items to other sub-galleries within the system.

You can modify the initial data directly in `db/init.sql` before starting, or through the Adminer interface while the system is running.

## 📱 Mobile Note
The interface is specially tailored for mobile browsers, solving common background "zooming" issues and ensuring navigation controls are always accessible at the bottom of the screen.
