use core::panic;
use std::fs::File;
use std::io::prelude::*;

fn get_all_links(filename: &str) -> Result<Vec<String>, std::io::Error> {
    let mut links_file = File::open(filename)?;

    let mut all_links = String::new();
    links_file.read_to_string(&mut all_links)?;

    let parts = all_links
        .trim_end_matches("\n")
        .split(", ")
        .map(|slice: &str| slice.to_string());

    Ok(parts.collect())
}

async fn request_album_data(link: &str) -> Result<(String, String), anyhow::Error> {
    let response = reqwest::get(link).await?.text().await?;

    let document = scraper::Html::parse_document(&response);

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

    match (album_title, album_art_link) {
        (Some(title), Some(link)) => Ok((title.to_string(), link.to_string())),
        _ => Err(anyhow::Error::msg(
            "Failed to find either the album art link or the album title.",
        )),
    }
}

async fn download_album_art_image(
    album_title: &str,
    album_art_link: &str,
) -> Result<(), anyhow::Error> {
    let response = reqwest::get(album_art_link).await?.bytes().await?;

    let mut file = File::create(format!("album_arts/{}.jpg", album_title))?;
    file.write_all(&response)?;

    Ok(())
}

async fn run(link: String) {
    match request_album_data(&link).await {
        Ok((album_title, album_art_link)) => {
            if let Err(error) = download_album_art_image(&album_title, &album_art_link).await {
                println!("{}", error);
            }
        }
        Err(error) => println!("{:?}\n", error),
    };
}

async fn request_all_album_pages(links: &[String]) {
    let mut set = tokio::task::JoinSet::new();

    for link in links.iter() {
        let link_clone = link.clone();
        set.spawn(async move { run(link_clone).await });
    }
    while let Some(res) = set.join_next().await {
        if let Err(error) = res {
            println!("{}", error);
        }
    }
}

fn main() {
    let mut links = get_all_links("links.txt").expect("A file named links.txt should be present");

    // use this function once the stabilsation is released.
    // links.pop_if(|link: &mut String| link.is_empty());
    if let Some("") = links.last().map(|link: &String| link.as_str()) {
        links.pop();
    }

    if links.is_empty() {
        panic!("No links in links.txt")
    }

    std::fs::create_dir_all("album_arts").expect("Failed to create album_arts directory");

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap()
        .block_on(request_all_album_pages(&links))
}
