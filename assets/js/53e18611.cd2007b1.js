(window.webpackJsonp=window.webpackJsonp||[]).push([[20],{128:function(e,t,n){"use strict";n.d(t,"a",(function(){return s})),n.d(t,"b",(function(){return f}));var r=n(0),o=n.n(r);function i(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function a(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,r)}return n}function u(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?a(Object(n),!0).forEach((function(t){i(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):a(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function c(e,t){if(null==e)return{};var n,r,o=function(e,t){if(null==e)return{};var n,r,o={},i=Object.keys(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||(o[n]=e[n]);return o}(e,t);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(o[n]=e[n])}return o}var d=o.a.createContext({}),l=function(e){var t=o.a.useContext(d),n=t;return e&&(n="function"==typeof e?e(t):u(u({},t),e)),n},s=function(e){var t=l(e.components);return o.a.createElement(d.Provider,{value:t},e.children)},b={inlineCode:"code",wrapper:function(e){var t=e.children;return o.a.createElement(o.a.Fragment,{},t)}},p=o.a.forwardRef((function(e,t){var n=e.components,r=e.mdxType,i=e.originalType,a=e.parentName,d=c(e,["components","mdxType","originalType","parentName"]),s=l(n),p=r,f=s["".concat(a,".").concat(p)]||s[p]||b[p]||i;return n?o.a.createElement(f,u(u({ref:t},d),{},{components:n})):o.a.createElement(f,u({ref:t},d))}));function f(e,t){var n=arguments,r=t&&t.mdxType;if("string"==typeof e||r){var i=n.length,a=new Array(i);a[0]=p;var u={};for(var c in t)hasOwnProperty.call(t,c)&&(u[c]=t[c]);u.originalType=e,u.mdxType="string"==typeof e?e:r,a[1]=u;for(var d=2;d<i;d++)a[d]=n[d];return o.a.createElement.apply(null,a)}return o.a.createElement.apply(null,n)}p.displayName="MDXCreateElement"},92:function(e,t,n){"use strict";n.r(t),n.d(t,"frontMatter",(function(){return a})),n.d(t,"metadata",(function(){return u})),n.d(t,"toc",(function(){return c})),n.d(t,"default",(function(){return l}));var r=n(3),o=n(7),i=(n(0),n(128)),a={id:"introduction",title:"BuildBuddy Docs",sidebar_label:"Introduction"},u={unversionedId:"introduction",id:"introduction",isDocsHomePage:!1,title:"BuildBuddy Docs",description:"BuildBuddy is an open-core Bazel build event viewer, result store, remote cache, and remote build execution platform.",source:"@site/../docs/introduction.md",slug:"/introduction",permalink:"/docs/introduction",editUrl:"https://github.com/buildbuddy-io/buildbuddy/edit/master/docs/../docs/introduction.md",version:"current",sidebar_label:"Introduction",sidebar:"someSidebar",next:{title:"Cloud Quickstart",permalink:"/docs/cloud"}},c=[{value:"Get started",id:"get-started",children:[]},{value:"Go further",id:"go-further",children:[]},{value:"Start contributing",id:"start-contributing",children:[]},{value:"Join the discussion",id:"join-the-discussion",children:[]}],d={toc:c};function l(e){var t=e.components,n=Object(o.a)(e,["components"]);return Object(i.b)("wrapper",Object(r.a)({},d,n,{components:t,mdxType:"MDXLayout"}),Object(i.b)("p",null,"BuildBuddy is an open-core Bazel build event viewer, result store, remote cache, and remote build execution platform."),Object(i.b)("h2",{id:"get-started"},"Get started"),Object(i.b)("p",null,"There are two main ways to get started with BuildBuddy:"),Object(i.b)("ol",null,Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/cloud"},"BuildBuddy Cloud"),": a fully managed cloud version of BuildBuddy that is free to use for individuals and open source projects. You can get up and running quickly by just adding a few lines to your ",Object(i.b)("inlineCode",{parentName:"li"},".bazelrc")," file."),Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/on-prem"},"BuildBuddy On-Prem"),": Run your own instance of BuildBuddy on your own servers or in your own cloud environment. Features targeted at individual developers are free and open source. ",Object(i.b)("a",{parentName:"li",href:"/docs/enterprise"},"BuildBuddy Enterprise")," is also available for companies that need advanced features like OIDC auth, API access, and more.")),Object(i.b)("h2",{id:"go-further"},"Go further"),Object(i.b)("p",null,"Once you've gotten started with BuildBuddy - there's lots more to check out!"),Object(i.b)("ol",null,Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/guides"},"Guides"),": Helpful guides to walk you through common BuildBuddy use-cases."),Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/config"},"Configuration options"),": Learn how to configure BuildBuddy to conform to your needs."),Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/remote-build-execution"},"Remote Build Execution"),": parallelize your builds across thousands of machines."),Object(i.b)("li",{parentName:"ol"},Object(i.b)("a",{parentName:"li",href:"/docs/troubleshooting"},"Troubleshooting"),": Where to go when you're stuck.")),Object(i.b)("h2",{id:"start-contributing"},"Start contributing"),Object(i.b)("p",null,"Check out our ",Object(i.b)("a",{parentName:"p",href:"/docs/contributing"},"contributing")," docs to find out how to get started contributing to BuildBuddy."),Object(i.b)("h2",{id:"join-the-discussion"},"Join the discussion"),Object(i.b)("p",null,"Join our ",Object(i.b)("a",{parentName:"p",href:"https://slack.buildbuddy.io"},"BuildBuddy Slack channel")," to talk to the team, ask questions, discuss BuildBuddy, and get to know us!"))}l.isMDXComponent=!0}}]);