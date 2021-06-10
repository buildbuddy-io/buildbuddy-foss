(window.webpackJsonp=window.webpackJsonp||[]).push([[53],{123:function(e,t,o){"use strict";o.r(t),o.d(t,"frontMatter",(function(){return c})),o.d(t,"metadata",(function(){return a})),o.d(t,"toc",(function(){return l})),o.d(t,"default",(function(){return s}));var r=o(3),n=o(7),i=(o(0),o(128)),c={id:"troubleshooting-rbe",title:"Troubleshooting RBE Failures",sidebar_label:"RBE Failures"},a={unversionedId:"troubleshooting-rbe",id:"troubleshooting-rbe",isDocsHomePage:!1,title:"Troubleshooting RBE Failures",description:"Remote connection/protocol failed with: execution failed",source:"@site/../docs/troubleshooting-rbe.md",slug:"/troubleshooting-rbe",permalink:"/docs/troubleshooting-rbe",editUrl:"https://github.com/buildbuddy-io/buildbuddy/edit/master/docs/../docs/troubleshooting-rbe.md",version:"current",sidebar_label:"RBE Failures",sidebar:"someSidebar",previous:{title:"Troubleshooting",permalink:"/docs/troubleshooting"},next:{title:"Troubleshooting Slow Uploads",permalink:"/docs/troubleshooting-slow-upload"}},l=[{value:"Remote connection/protocol failed with: execution failed",id:"remote-connectionprotocol-failed-with-execution-failed",children:[]},{value:"Remote connection/protocol failed with: execution failed DEADLINE_EXCEEDED: deadline exceeded after 59999899500ns",id:"remote-connectionprotocol-failed-with-execution-failed-deadline_exceeded-deadline-exceeded-after-59999899500ns",children:[]},{value:"exec user process caused &quot;exec format error&quot;",id:"exec-user-process-caused-exec-format-error",children:[]},{value:"rpc error: code = Unavailable desc = No registered executors.",id:"rpc-error-code--unavailable-desc--no-registered-executors",children:[]}],u={toc:l};function s(e){var t=e.components,o=Object(n.a)(e,["components"]);return Object(i.b)("wrapper",Object(r.a)({},u,o,{components:t,mdxType:"MDXLayout"}),Object(i.b)("h2",{id:"remote-connectionprotocol-failed-with-execution-failed"},"Remote connection/protocol failed with: execution failed"),Object(i.b)("p",null,"This error is often a sign that a cache write is timing out. By default, bazel's ",Object(i.b)("inlineCode",{parentName:"p"},"remote_timeout")," ",Object(i.b)("a",{parentName:"p",href:"https://docs.bazel.build/versions/master/command-line-reference.html#flag--remote_timeout"},"flag")," limits all remote execution calls to 60 seconds."),Object(i.b)("p",null,"We recommend using the following flag to increase this remote timeout:"),Object(i.b)("pre",null,Object(i.b)("code",{parentName:"pre"},"--remote_timeout=600\n")),Object(i.b)("p",null,"These expensive writes should only happen once when artifacts are initially written to the cache, and shouldn't happen on subsequent builds."),Object(i.b)("h2",{id:"remote-connectionprotocol-failed-with-execution-failed-deadline_exceeded-deadline-exceeded-after-59999899500ns"},"Remote connection/protocol failed with: execution failed DEADLINE_EXCEEDED: deadline exceeded after 59999899500ns"),Object(i.b)("p",null,"This error is a sign that a cache write is timing out. By default, bazel's ",Object(i.b)("inlineCode",{parentName:"p"},"remote_timeout")," ",Object(i.b)("a",{parentName:"p",href:"https://docs.bazel.build/versions/master/command-line-reference.html#flag--remote_timeout"},"flag")," limits all remote execution calls to 60 seconds."),Object(i.b)("p",null,"We recommend using the following flag to increase this remote timeout:"),Object(i.b)("pre",null,Object(i.b)("code",{parentName:"pre"},"--remote_timeout=600\n")),Object(i.b)("h2",{id:"exec-user-process-caused-exec-format-error"},'exec user process caused "exec format error"'),Object(i.b)("p",null,"This error occurs when your build is configured for darwin (Mac OSX) CPUs. BuildBuddy Cloud currently doesn't run Mac executors, but plan to in the coming months."),Object(i.b)("p",null,"In the meantime, you can configure your toolchains to target k8 - or execute your builds from a linux host using either Docker or a CI system of your choice."),Object(i.b)("h2",{id:"rpc-error-code--unavailable-desc--no-registered-executors"},"rpc error: code = Unavailable desc = No registered executors."),Object(i.b)("p",null,"This error occurs when your build is configured for darwin (Mac OSX) CPUs. BuildBuddy Cloud currently doesn't run Mac executors, but plan to in the coming months."),Object(i.b)("p",null,"In the meantime, you can configure your toolchains to target k8 - or execute your builds from a linux host using either Docker or a CI system of your choice."))}s.isMDXComponent=!0},128:function(e,t,o){"use strict";o.d(t,"a",(function(){return d})),o.d(t,"b",(function(){return m}));var r=o(0),n=o.n(r);function i(e,t,o){return t in e?Object.defineProperty(e,t,{value:o,enumerable:!0,configurable:!0,writable:!0}):e[t]=o,e}function c(e,t){var o=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),o.push.apply(o,r)}return o}function a(e){for(var t=1;t<arguments.length;t++){var o=null!=arguments[t]?arguments[t]:{};t%2?c(Object(o),!0).forEach((function(t){i(e,t,o[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(o)):c(Object(o)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(o,t))}))}return e}function l(e,t){if(null==e)return{};var o,r,n=function(e,t){if(null==e)return{};var o,r,n={},i=Object.keys(e);for(r=0;r<i.length;r++)o=i[r],t.indexOf(o)>=0||(n[o]=e[o]);return n}(e,t);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(e);for(r=0;r<i.length;r++)o=i[r],t.indexOf(o)>=0||Object.prototype.propertyIsEnumerable.call(e,o)&&(n[o]=e[o])}return n}var u=n.a.createContext({}),s=function(e){var t=n.a.useContext(u),o=t;return e&&(o="function"==typeof e?e(t):a(a({},t),e)),o},d=function(e){var t=s(e.components);return n.a.createElement(u.Provider,{value:t},e.children)},b={inlineCode:"code",wrapper:function(e){var t=e.children;return n.a.createElement(n.a.Fragment,{},t)}},p=n.a.forwardRef((function(e,t){var o=e.components,r=e.mdxType,i=e.originalType,c=e.parentName,u=l(e,["components","mdxType","originalType","parentName"]),d=s(o),p=r,m=d["".concat(c,".").concat(p)]||d[p]||b[p]||i;return o?n.a.createElement(m,a(a({ref:t},u),{},{components:o})):n.a.createElement(m,a({ref:t},u))}));function m(e,t){var o=arguments,r=t&&t.mdxType;if("string"==typeof e||r){var i=o.length,c=new Array(i);c[0]=p;var a={};for(var l in t)hasOwnProperty.call(t,l)&&(a[l]=t[l]);a.originalType=e,a.mdxType="string"==typeof e?e:r,c[1]=a;for(var u=2;u<i;u++)c[u]=o[u];return n.a.createElement.apply(null,c)}return n.a.createElement.apply(null,o)}p.displayName="MDXCreateElement"}}]);