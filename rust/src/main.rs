use little_exif::exif_tag::ExifTag;
use little_exif::metadata::Metadata;
use std::collections::HashSet;
use std::fs::File;
use std::io::prelude::*;

const YOUTUBE_MUSIC_ALBUM_CODE_LENGTH: usize = 11;

fn trim_album_code_from_link(link: &str) -> &str {
    let split_position = link
        .char_indices()
        .nth_back(YOUTUBE_MUSIC_ALBUM_CODE_LENGTH)
        .unwrap()
        .0;

    &link[split_position..]
}

fn get_all_links(filename: &str) -> Result<Vec<String>, anyhow::Error> {
    let mut links_file = File::open(filename)?;

    let mut all_links = String::new();
    links_file.read_to_string(&mut all_links)?;

    let mut links: Vec<String> = all_links
        .trim_end_matches("\n")
        .split(", ")
        .map(|slice: &str| slice.to_string())
        .collect();

    // use this function once the stabilsation is released.
    // links.pop_if(|link: &mut String| link.is_empty());
    if let Some("") = links.last().map(|link: &String| link.as_str()) {
        links.pop();
    }

    if links.is_empty() {
        return Err(anyhow::Error::msg("No links in links.txt"));
    }

    Ok(links)
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
    youtube_album_code: &str,
    album_art_directory_name: &str,
) -> Result<(), anyhow::Error> {
    let mut response = reqwest::get(album_art_link).await?.bytes().await?.to_vec();

    let mut metadata = Metadata::new();
    metadata.set_tag(ExifTag::ImageDescription(youtube_album_code.to_string()));
    metadata.write_to_vec(&mut response, little_exif::filetype::FileExtension::JPEG)?;

    let mut file = File::create(format!("{album_art_directory_name}/{album_title}.jpg"))?;
    file.write_all(&response)?;

    Ok(())
}

async fn run(link: String, album_art_directory_name: &str) {
    match request_album_data(&link).await {
        Ok((album_title, album_art_link)) => {
            let album_code = trim_album_code_from_link(&link);
            if let Err(error) = download_album_art_image(
                &album_title,
                &album_art_link,
                album_code,
                album_art_directory_name,
            )
            .await
            {
                eprintln!("{error:?}");
            }
        }
        Err(error) => eprintln!("{error:?}"),
    };
}

async fn request_all_album_pages(links: &[String], album_art_directory_name: &'static str) {
    let mut set = tokio::task::JoinSet::new();

    for link in links.iter() {
        let link_clone = link.clone();
        set.spawn(async move { run(link_clone, album_art_directory_name).await });
    }
    while let Some(res) = set.join_next().await {
        if let Err(error) = res {
            eprintln!("{error:?}");
        }
    }
}

fn get_codes_of_existing_album_art(
    album_art_directory: &str,
) -> Result<HashSet<String>, anyhow::Error> {
    let mut existing_images: HashSet<String> = HashSet::new();

    let directory = std::fs::read_dir(album_art_directory)?;
    for file in directory {
        let metadata = Metadata::new_from_path(&file?.path())?;

        let mut exif_iterator = metadata.get_tag(&ExifTag::ImageDescription(String::from("hello")));
        if let Some(album_code) = exif_iterator.next() {
            let mut bytes = album_code.value_as_u8_vec(&little_exif::endian::Endian::Little);
            bytes.pop();
            existing_images.insert(String::from_utf8(bytes)?);
        }
    }

    Ok(existing_images)
}

fn get_links_to_download(
    links_file_name: &str,
    album_art_directory_name: &str,
) -> Result<Vec<String>, anyhow::Error> {
    let all_links = get_all_links(links_file_name)?;
    let existing_album_codes = get_codes_of_existing_album_art(album_art_directory_name)?;

    let mut links_to_download: Vec<String> = Vec::new();
    for link in all_links {
        let album_code = trim_album_code_from_link(&link);
        if existing_album_codes.contains(album_code) {
            continue;
        }
        links_to_download.push(link);
    }

    Ok(links_to_download)
}

fn main() {
    let links_to_download = match get_links_to_download("links.txt", "album_arts") {
        Ok(links) => links,
        Err(error) => panic!("{error:?}"),
    };

    std::fs::create_dir_all("album_arts").expect("Failed to create album_arts directory");

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap()
        .block_on(request_all_album_pages(&links_to_download, "album_arts"))
}
