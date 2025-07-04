import Image from "@theme/IdealImage";
import React from "react";
import styles from "./brands.module.css";

function Component() {
  return (
    <div className={styles.brands}>
      <a target="_blank" href="https://www.ycombinator.com/companies/buildbuddy">
        <Image
          alt="Y Combinator Logo"
          className={styles.image}
          img={require("../../../static/img/yc_monochrome.png")}
          shouldAutoDownload={() => true}
          threshold={10000}
        />
      </a>
      <a target="_blank" href="https://cloud.google.com/find-a-partner/partner/buildbuddy">
        <Image
          alt="Google Cloud Partner Logo"
          className={styles.image}
          img={require("../../../static/img/gcp_monochrome.png")}
          shouldAutoDownload={() => true}
          threshold={10000}
        />
      </a>
      <a
        target="_blank"
        href="https://techcrunch.com/2020/12/01/yc-backed-buildbuddy-raises-3-15m-to-help-developers-build-software-more-quickly/">
        <Image
          alt="Techcrunch Logo"
          className={styles.image}
          img={require("../../../static/img/tc_monochrome.png")}
          shouldAutoDownload={() => true}
          threshold={10000}
        />
      </a>
    </div>
  );
}

export default Component;
