<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet
    version="1.0"
    xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    
    <xsl:output method="xml" encoding="UTF-8" indent="yes" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>

    <!-- Root -->
    <xsl:template match="/document">
        <html>
            <head>
                <meta charset="utf-8" />
                <meta name="viewport" content="width=device-width" />
                <link rel="icon" type="image/x-icon" href="/favicon.ico" />
                <title><xsl:value-of select="meta/title/@value"/></title>
            </head>
            <body>
                <xsl:apply-templates select="body/*"/>
            </body>
        </html>
    </xsl:template>
    
    <!-- TEXT -->
    <!-- Trim leading/trailing whitespace, keep internal formatting -->
    <xsl:template match="text">
        <p>
            <xsl:value-of select="normalize-space(
                    concat(
                        substring(., 1, 1),
                        substring(., 2)
                    )
                )"/>
        </p>
    </xsl:template>
    
    <!-- LINK -->
    <xsl:template match="link">
        <a href="{@href}"><xsl:value-of select="."/></a><br/>
    </xsl:template>
    
    <!-- BOLD -->
    <xsl:template match="bold">
        <strong><p><xsl:value-of select="."/></p></strong>
    </xsl:template>
    
    <!-- CODE -->
    <xsl:template match="code">
        <xsl:choose>
            
            <!-- Code contains a table -->
            <xsl:when test="table">
                <xsl:apply-templates select="table"/>
            </xsl:when>
            
            <!-- Plain code -->
            <xsl:otherwise>
                <pre><code>
                        <xsl:value-of select="normalize-space(.)"/>
                    </code></pre>
            </xsl:otherwise>
            
        </xsl:choose>
    </xsl:template>
    
    <xsl:template match="table">
        <table>
            <xsl:apply-templates/>
        </table>
    </xsl:template>
    
    <xsl:template match="tr">
        <tr><xsl:apply-templates/></tr>
    </xsl:template>
    
    <xsl:template match="td">
        <td>
            <xsl:if test="@style">
                <xsl:attribute name="style">
                    <xsl:value-of select="@style"/>
                </xsl:attribute>
            </xsl:if>
            <xsl:value-of select="."/>
        </td>
    </xsl:template>
    
    <!-- FIRST ITEM IN A SEQUENCE -->
    <xsl:template match="item[not(preceding-sibling::*[1][self::item])]">
        <ul>
            <xsl:apply-templates
                select=". | following-sibling::*[
                        self::item
                        and
                        generate-id(preceding-sibling::*[not(self::item)][1]) 
                        = generate-id(current()/preceding-sibling::*[not(self::item)][1])
                    ]"
                mode="item-group"/>
        </ul>
    </xsl:template>
    
    <xsl:template match="item"/>
    
    <xsl:template match="item" mode="item-group">
        <li>
            <xsl:value-of select="."/>
        </li>
    </xsl:template>
    
</xsl:stylesheet>
