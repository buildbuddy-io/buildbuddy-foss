import Link from "@docusaurus/Link";
import { PageMetadata } from "@docusaurus/theme-common";
import Translate, { translate } from "@docusaurus/Translate";
import { MDXProvider } from "@mdx-js/react";
import type { Props } from "@theme/BlogPostItem";
import MDXComponents from "@theme/MDXComponents";
import clsx from "clsx";
import React from "react";

import styles from "./styles.module.css";

const MONTHS = [
  translate({
    id: "theme.common.month.january",
    description: "January month translation",
    message: "January",
  }),
  translate({
    id: "theme.common.month.february",
    description: "February month translation",
    message: "February",
  }),
  translate({
    id: "theme.common.month.march",
    description: "March month translation",
    message: "March",
  }),
  translate({
    id: "theme.common.month.april",
    description: "April month translation",
    message: "April",
  }),
  translate({
    id: "theme.common.month.may",
    description: "May month translation",
    message: "May",
  }),
  translate({
    id: "theme.common.month.june",
    description: "June month translation",
    message: "June",
  }),
  translate({
    id: "theme.common.month.july",
    description: "July month translation",
    message: "July",
  }),
  translate({
    id: "theme.common.month.august",
    description: "August month translation",
    message: "August",
  }),
  translate({
    id: "theme.common.month.september",
    description: "September month translation",
    message: "September",
  }),
  translate({
    id: "theme.common.month.october",
    description: "October month translation",
    message: "October",
  }),
  translate({
    id: "theme.common.month.november",
    description: "November month translation",
    message: "November",
  }),
  translate({
    id: "theme.common.month.december",
    description: "December month translation",
    message: "December",
  }),
];

function BlogPostItem(props: Props): JSX.Element {
  const { children, frontMatter, metadata, truncated, isBlogPostPage = false } = props;
  const { date, permalink, tags, readingTime } = metadata;
  const { title, subtitle, image, keywords } = frontMatter;

  // TODO: support multiple authors
  const author = metadata.authors[0];
  const { name, title: authorTitle, url, imageURL } = author;

  const renderPostHeader = () => {
    const TitleHeading = isBlogPostPage ? "h1" : "h2";
    const SubtitleHeading = isBlogPostPage ? "h2" : "h3";
    const match = date.substring(0, 10).split("-");
    const year = match[0];
    const month = MONTHS[parseInt(match[1], 10) - 1];
    const day = parseInt(match[2], 10);

    return (
      <header>
        <TitleHeading
          className={clsx("margin-bottom--sm", isBlogPostPage ? styles.blogPostTitle : styles.blogPostTitleGrid)}>
          {isBlogPostPage ? title : <Link to={permalink}>{title}</Link>}
        </TitleHeading>
        {subtitle && <SubtitleHeading className={styles.subtitle}>{subtitle}</SubtitleHeading>}
        <div className="margin-vert--md">
          <div className={styles.heading}>
            <div className={styles.headingPhoto}>
              {imageURL && (
                <Link className={`avatar__photo-link avatar__photo ${styles.avatarImage}`} href={imageURL}>
                  <img src={imageURL} alt={author} />
                </Link>
              )}
            </div>
            <div className={styles.headingDetails}>
              <span>
                <Link className={styles.authorName} href={url}>
                  {name}
                </Link>
                {", "}
                <span className={styles.authorTitle}>{authorTitle}</span>
              </span>
              <time dateTime={date} className={styles.blogPostDate}>
                <br />
                <Translate
                  id="theme.blog.post.date"
                  description="The label to display the blog post date"
                  values={{ day, month, year }}>
                  {"{month} {day}, {year}"}
                </Translate>{" "}
                {readingTime && (
                  <>
                    {" · "}
                    <Translate
                      id="theme.blog.post.readingTime"
                      description="The label to display reading time of the blog post"
                      values={{
                        readingTime: Math.ceil(readingTime),
                      }}>
                      {"{readingTime} min read"}
                    </Translate>
                  </>
                )}
              </time>
            </div>
          </div>
        </div>
      </header>
    );
  };

  return (
    <>
      <PageMetadata {...{ keywords, image }} />

      <article className={!isBlogPostPage ? `${styles.articleGrid}` : undefined}>
        {renderPostHeader()}
        {
          <div className="markdown">
            <MDXProvider components={MDXComponents}>{children}</MDXProvider>
          </div>
        }
        {(tags.length > 0 || truncated) && isBlogPostPage && (
          <footer className="row margin-vert--lg">
            {tags.length > 0 && !truncated && (
              <div className={styles.tags}>
                <strong>
                  <Translate id="theme.tags.tagsListLabel" description="The label alongside a tag list">
                    Tags:
                  </Translate>
                </strong>
                {tags.map(({ label, permalink: tagPermalink }) => (
                  <Link key={tagPermalink} className={styles.tag} to={tagPermalink}>
                    {label}
                  </Link>
                ))}
              </div>
            )}
          </footer>
        )}
      </article>
    </>
  );
}

export default BlogPostItem;
