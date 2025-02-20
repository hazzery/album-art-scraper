use clap::Parser;
use little_exif::exif_tag::ExifTag;
use little_exif::metadata::Metadata;
use std::collections::HashSet;
use std::fs::File;
use std::io::prelude::*;

/// Download album art images of albums specified by links in file.
#[derive(Parser)]
struct Cli {
    /// The file to find album links in.
    #[arg(short, long, default_value = "links.txt")]
    links_file: std::path::PathBuf,

    /// The directory to download album art images to.
    #[arg(short, long, default_value = "album_arts")]
    image_directory: std::path::PathBuf,
}

const YOUTUBE_MUSIC_ALBUM_CODE_LENGTH: usize = 11;

fn trim_album_code_from_link(link: &str) -> &str {
    let split_position = link
        .char_indices()
        .nth_back(YOUTUBE_MUSIC_ALBUM_CODE_LENGTH)
        .unwrap()
        .0;

    &link[split_position..]
}

fn get_all_links(filename: &std::path::Path) -> Result<Vec<String>, anyhow::Error> {
    let file = std::path::Path::new(filename);

    if !file.exists() {
        return Ok(Vec::new());
    }

    if !file.is_file() {
        return Err(anyhow::Error::msg(format!(
            "Unable to read links from `{}` as it is not a file.",
            filename.to_string_lossy(),
        )));
    }
    let mut links_file = File::open(file)?;

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
    album_art_directory_name: &std::path::Path,
) -> Result<(), anyhow::Error> {
    println!("Downloading image for {}", album_title);

    let mut response = reqwest::get(album_art_link).await?.bytes().await?.to_vec();

    let mut metadata = Metadata::new();
    metadata.set_tag(ExifTag::ImageDescription(youtube_album_code.to_string()));
    metadata.write_to_vec(&mut response, little_exif::filetype::FileExtension::JPEG)?;

    let mut file = File::create(
        album_art_directory_name
            .join(album_title.replace("/", " "))
            .with_extension("jpg"),
    )?;
    file.write_all(&response)?;

    Ok(())
}

async fn run(link: String, album_art_directory_name: &std::path::Path) {
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

async fn request_all_album_pages(links: &[String], album_art_directory_name: std::path::PathBuf) {
    let album_art_directory_name = std::sync::Arc::new(album_art_directory_name);
    let mut set = tokio::task::JoinSet::new();

    for link in links.iter() {
        let link_clone = link.clone();
        let album_art_directory_name_clone = std::sync::Arc::clone(&album_art_directory_name);
        set.spawn(async move { run(link_clone, &album_art_directory_name_clone).await });
    }
    while let Some(res) = set.join_next().await {
        if let Err(error) = res {
            eprintln!("{error:?}");
        }
    }
}

fn get_codes_of_existing_album_art(
    album_art_directory: &std::path::Path,
) -> Result<HashSet<String>, anyhow::Error> {
    let directory = std::path::Path::new(album_art_directory);
    if !directory.exists() {
        std::fs::create_dir_all(directory)?;
        return Ok(HashSet::new());
    }

    if !directory.is_dir() {
        return Err(anyhow::Error::msg(format!(
            "Unable to read album art images from `{}` as it is not a directory.",
            album_art_directory.to_string_lossy(),
        )));
    }

    let mut existing_images: HashSet<String> = HashSet::new();

    for file in std::fs::read_dir(directory)? {
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
    links_file_name: &std::path::Path,
    album_art_directory_name: &std::path::Path,
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
    let args = Cli::parse();

    let links_to_download = match get_links_to_download(&args.links_file, &args.image_directory) {
        Ok(links) => links,
        Err(error) => {
            eprintln!("{error:?}");
            std::process::exit(1);
        }
    };

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap()
        .block_on(request_all_album_pages(
            &links_to_download,
            args.image_directory,
        ))
}
