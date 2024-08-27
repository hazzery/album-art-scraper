const atags = document.getElementsByClassName("yt-simple-endpoint image-wrapper style-scope ytmusic-two-row-item-renderer");
let hrefs = [];
for (const atag of atags) {
  hrefs.push(atag.href);
}
console.log(hrefs.join(", "));

