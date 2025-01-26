use std::fs;
use std::io::prelude::*;
use std::{fs::File, io};

fn get_all_links(filename: &str) -> Result<Vec<String>, io::Error> {
    let mut links_file = File::open(filename)?;

    let mut all_links = String::new();
    links_file.read_to_string(&mut all_links)?;

    let parts = all_links
        .trim_end_matches("\n")
        .split(",")
        .map(|slice| slice.to_string());

    Ok(parts.collect())
}

async fn request_album_data(link: &str) -> Option<(String, String)> {
    let response = match reqwest::get(link).await {
        Err(error) => {
            println!("{}", error);
            return None;
        }
        Ok(response) => response,
    };

    let response_text = match response.text().await {
        Err(error) => {
            println!("Response text gave error: {}", error);
            return None;
        }
        Ok(response_text) => response_text,
    };

    let document = scraper::Html::parse_document(&response_text);

    let mut album_art_link: Option<&str> = None;
    let mut album_title: Option<&str> = None;

    let selector = scraper::Selector::parse("meta").unwrap();
    for element in document.select(&selector) {
        match element.attr("property") {
            Some("og:image") => album_art_link = element.attr("content"),
            Some("og:title") => album_title = element.attr("content"),
            _ => (),
        };
    }

    if let (Some(title), Some(link)) = (album_title, album_art_link) {
        Some((title.to_string(), link.to_string()))
    } else {
        println!("Could not find album title or album art link");
        None
    }
}

async fn download_album_art_image(album_title: &str, album_art_link: &str) {
    let response = match reqwest::get(album_art_link).await {
        Err(error) => {
            println!(
                "Request to {} gave error response: {}",
                album_art_link, error
            );
            return;
        }
        Ok(response) => response,
    };

    let response_bytes = match response.bytes().await {
        Err(error) => {
            println!("Response bytes gave error: {}", error);
            return;
        }
        Ok(response_bytes) => response_bytes,
    };

    let mut file = match File::create(format!("album_arts/{}.jpg", album_title)) {
        Err(error) => {
            println!("Creating file failed with error: {}", error);
            return;
        }
        Ok(file) => file,
    };

    if let Err(error) = file.write_all(&response_bytes) {
        println!("Writing image to file failed with error: {}", error);
    }
}

async fn request_all_album_pages(links: &[String]) {
    let mut tasks = Vec::new();

    for link in links.iter() {
        let link_clone = link.clone();
        let task = tokio::spawn(async move {
            if let Some((album_title, album_art_link)) = request_album_data(&link_clone).await {
                download_album_art_image(&album_title, &album_art_link).await
            };
        });

        tasks.push(task); // Store the task
    }
}

fn main() {
    let links = get_all_links("links.txt").expect("No links.txt file present");
    fs::create_dir_all("album_arts").expect("Failed to create album_arts directory");

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap()
        .block_on(request_all_album_pages(&links))
}
